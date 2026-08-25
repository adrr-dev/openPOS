package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

type CategoryRepo struct {
	pool *pgxpool.Pool
}

func NewCategoryRepo(pool *pgxpool.Pool) *CategoryRepo { return &CategoryRepo{pool: pool} }

const catCols = `id, store_id, name, active, created_at`

func (r *CategoryRepo) ListByStore(ctx context.Context, storeID string) ([]*model.Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+catCols+` FROM categories WHERE store_id = $1 ORDER BY created_at ASC
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.StoreID, &c.Name, &c.Active, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *CategoryRepo) GetByID(ctx context.Context, storeID, id string) (*model.Category, error) {
	var c model.Category
	err := r.pool.QueryRow(ctx, `
		SELECT `+catCols+` FROM categories WHERE id = $1 AND store_id = $2
	`, id, storeID).Scan(&c.ID, &c.StoreID, &c.Name, &c.Active, &c.CreatedAt)
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &c, nil
}

func (r *CategoryRepo) Create(ctx context.Context, storeID, name string) (*model.Category, error) {
	var c model.Category
	err := r.pool.QueryRow(ctx, `
		INSERT INTO categories (store_id, name) VALUES ($1, $2) RETURNING `+catCols+`
	`, storeID, name).Scan(&c.ID, &c.StoreID, &c.Name, &c.Active, &c.CreatedAt)
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &c, nil
}

// Delete menghapus kategori. Return softDeleted=true bila masih dipakai produk
// sehingga hanya dinonaktifkan (FR-CAT-002), false bila benar-benar dihapus.
func (r *CategoryRepo) Delete(ctx context.Context, storeID, id string) (softDeleted bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1 AND store_id = $2)`, id, storeID,
	).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, ErrNotFound
	}

	var used bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM products WHERE category_id = $1 AND store_id = $2)`, id, storeID,
	).Scan(&used); err != nil {
		return false, err
	}
	if used {
		if _, err := tx.Exec(ctx,
			`UPDATE categories SET active = FALSE WHERE id = $1 AND store_id = $2`, id, storeID,
		); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}

	tag, err := tx.Exec(ctx, `DELETE FROM categories WHERE id = $1 AND store_id = $2`, id, storeID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return false, nil
}
