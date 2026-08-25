package repo

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

type ProductFilter struct {
	Q          string // cari di name/sku/barcode
	CategoryID string
	Active     *bool
	Page       int // mulai 1
	Limit      int // default 20, maks 200
}

type ProductPage struct {
	Items []*model.Product `json:"items"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

const prodCols = `p.id, p.store_id, p.category_id, c.name, p.name, p.sku, p.barcode,
	p.buy_price, p.sell_price, p.stock, p.unit, p.active, p.created_at`

func scanProduct(row interface{ Scan(...any) error }) (*model.Product, error) {
	var p model.Product
	if err := row.Scan(&p.ID, &p.StoreID, &p.CategoryID, &p.CategoryName, &p.Name, &p.SKU,
		&p.Barcode, &p.BuyPrice, &p.SellPrice, &p.Stock, &p.Unit, &p.Active, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

type ProductRepo struct {
	pool *pgxpool.Pool
}

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo { return &ProductRepo{pool: pool} }

// GetByID mengambil produk dalam toko tertentu.
func (r *ProductRepo) GetByID(ctx context.Context, storeID, id string) (*model.Product, error) {
	p, err := scanProduct(r.pool.QueryRow(ctx, `
		SELECT `+prodCols+`
		FROM products p LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.id = $1 AND p.store_id = $2
	`, id, storeID))
	if err != nil {
		return nil, mapDBErr(err)
	}
	return p, nil
}

// List mengembalikan halaman produk sesuai filter (server-side pagination, NFR-008).
func (r *ProductRepo) List(ctx context.Context, storeID string, f ProductFilter) (*ProductPage, error) {
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	conds := []string{"p.store_id = $1"}
	args := []any{storeID}
	if q := strings.TrimSpace(f.Q); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		n := len(args)
		conds = append(conds,
			`(lower(p.name) LIKE $`+strconv.Itoa(n)+
				` OR lower(p.sku) LIKE $`+strconv.Itoa(n)+
				` OR lower(p.barcode) LIKE $`+strconv.Itoa(n)+`)`)
	}
	if f.CategoryID != "" {
		args = append(args, f.CategoryID)
		conds = append(conds, `p.category_id = $`+strconv.Itoa(len(args)))
	}
	if f.Active != nil {
		args = append(args, *f.Active)
		conds = append(conds, `p.active = $`+strconv.Itoa(len(args)))
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM products p`+where, args...,
	).Scan(&total); err != nil {
		return nil, err
	}

	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	args = append(args, limit, (page-1)*limit)

	rows, err := r.pool.Query(ctx, `
		SELECT `+prodCols+`
		FROM products p LEFT JOIN categories c ON c.id = p.category_id
		`+where+`
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $`+strconv.Itoa(limitIdx)+` OFFSET $`+strconv.Itoa(offsetIdx), args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*model.Product, 0, limit)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ProductPage{Items: items, Total: total, Page: page, Limit: limit}, nil
}

// Create membuat produk baru. SKU duplikat per toko ditolak oleh unique index.
func (r *ProductRepo) Create(ctx context.Context, storeID string, p *model.Product) (*model.Product, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO products (store_id, category_id, name, sku, barcode, buy_price, sell_price, stock, unit)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, storeID, p.CategoryID, p.Name, p.SKU, p.Barcode, p.BuyPrice, p.SellPrice, p.Stock, p.Unit).Scan(&id)
	if err != nil {
		return nil, mapDBErr(err)
	}
	return r.GetByID(ctx, storeID, id)
}

// Update memperbarui atribut produk TANPA menyentuh stok (stok hanya lewat penyesuaian/transaksi).
func (r *ProductRepo) Update(ctx context.Context, storeID, id string, p *model.Product) (*model.Product, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE products SET
			category_id = $3, name = $4, sku = $5, barcode = $6,
			buy_price = $7, sell_price = $8, unit = $9
		WHERE id = $1 AND store_id = $2
	`, id, storeID, p.CategoryID, p.Name, p.SKU, p.Barcode, p.BuyPrice, p.SellPrice, p.Unit)
	if err != nil {
		return nil, mapDBErr(err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, storeID, id)
}

func (r *ProductRepo) SetActive(ctx context.Context, storeID, id string, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE products SET active = $3 WHERE id = $1 AND store_id = $2`, id, storeID, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// IsSkuTaken mengecek keambilan SKU pada toko (abaikan produk tertentu, untuk update).
func (r *ProductRepo) IsSkuTaken(ctx context.Context, storeID, sku, exceptProductID string) (bool, error) {
	var taken bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM products WHERE store_id = $1 AND lower(sku) = lower($2) AND id <> $3)
	`, storeID, strings.TrimSpace(sku), exceptProductID).Scan(&taken)
	return taken, err
}

// CreateWithInitialMovement membuat produk + movement 'initial' (bila stok awal > 0)
// dalam satu transaksi (FR-INV-003).
func (r *ProductRepo) CreateWithInitialMovement(ctx context.Context, storeID string, p *model.Product, actor string) (*model.Product, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO products (store_id, category_id, name, sku, barcode, buy_price, sell_price, stock, unit)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, storeID, p.CategoryID, p.Name, p.SKU, p.Barcode, p.BuyPrice, p.SellPrice, p.Stock, p.Unit).Scan(&id)
	if err != nil {
		return nil, mapDBErr(err)
	}
	if p.Stock > 0 {
		mv := &model.Movement{
			StoreID:   storeID,
			ProductID: id,
			Type:      model.MovementInitial,
			Qty:       p.Stock,
			Reason:    "Stok awal",
			Actor:     actor,
		}
		if err := insertMovementTx(ctx, tx, mv); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, storeID, id)
}

// ErrStockInsufficient: penyesuaian/transaksi akan membuat stok negatif.
var ErrStockInsufficient = errors.New("stok tidak cukup")

// AdjustStock mengubah stok atomik dan mencatat movement 'adjust' dalam transaksi
// yang sama (FR-INV-002 s.d. FR-INV-006). delta>0 menambah, delta<0 mengurangi.
func (r *ProductRepo) AdjustStock(ctx context.Context, storeID, productID string, delta int, reason, actor string) (*model.Product, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		UPDATE products SET stock = stock + $3
		WHERE id = $1 AND store_id = $2 AND stock + $3 >= 0
		RETURNING id
	`, productID, storeID, delta).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// bedakan: produk tak ada vs stok jadi negatif
		var exists bool
		if qerr := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND store_id = $2)`, productID, storeID,
		).Scan(&exists); qerr != nil {
			return nil, qerr
		}
		if exists {
			return nil, ErrStockInsufficient
		}
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, mapDBErr(err)
	}

	if err := insertMovementTx(ctx, tx, &model.Movement{
		StoreID:   storeID,
		ProductID: id,
		Type:      model.MovementAdjust,
		Qty:       delta,
		Reason:    reason,
		Actor:     actor,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, storeID, id)
}
