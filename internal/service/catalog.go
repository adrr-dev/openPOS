package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/0xMinomus/openPOS/backend/internal/model"
	"github.com/0xMinomus/openPOS/backend/internal/repo"
)

var (
	ErrCategoryInvalid = errors.New("kategori tidak ditemukan di toko Anda")
	ErrCategoryTaken   = errors.New("kategori dengan nama itu sudah ada")
	ErrSkuTaken        = errors.New("SKU sudah digunakan di toko ini")
)

type CatalogService struct {
	cats  *repo.CategoryRepo
	prods *repo.ProductRepo
	movs  *repo.MovementRepo
}

func NewCatalogService(cats *repo.CategoryRepo, prods *repo.ProductRepo, movs *repo.MovementRepo) *CatalogService {
	return &CatalogService{cats: cats, prods: prods, movs: movs}
}

// ── kategori ─────────────────────────────────────────────────────────

func (s *CatalogService) ListCategories(ctx context.Context, storeID string) ([]*model.Category, error) {
	return s.cats.ListByStore(ctx, storeID)
}

func (s *CatalogService) CreateCategory(ctx context.Context, storeID, name string) (*model.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("nama kategori wajib diisi")
	}
	c, err := s.cats.Create(ctx, storeID, name)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.Is(err, repo.ErrDuplicate) || (errors.As(err, &pgErr) && pgErr.Code == "23505") {
			return nil, ErrCategoryTaken
		}
		return nil, err
	}
	return c, nil
}

// DeleteCategory menghapus kategori: bila masih dipakai produk → soft-delete (FR-CAT-002).
func (s *CatalogService) DeleteCategory(ctx context.Context, storeID, id string) (soft bool, err error) {
	return s.cats.Delete(ctx, storeID, id)
}

// ── produk ───────────────────────────────────────────────────────────

type ProductInput struct {
	Name       string
	SKU        string
	Barcode    string
	CategoryID *string
	BuyPrice   int64
	SellPrice  int64
	Stock      int
	Unit       string
}

func normalizeProductInput(in *ProductInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.SKU = strings.TrimSpace(in.SKU)
	in.Barcode = strings.TrimSpace(in.Barcode)
	in.Unit = strings.TrimSpace(in.Unit)
	if in.Unit == "" {
		in.Unit = "pcs"
	}
	if in.Name == "" {
		return fmt.Errorf("nama produk wajib diisi")
	}
	if in.SKU == "" {
		return fmt.Errorf("SKU wajib diisi")
	}
	if in.BuyPrice < 0 || in.SellPrice < 0 {
		return fmt.Errorf("harga tidak boleh negatif")
	}
	if in.Stock < 0 {
		return fmt.Errorf("stok tidak boleh negatif")
	}
	return nil
}

func isPgUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *CatalogService) CreateProduct(ctx context.Context, storeID, actorName string, in ProductInput) (*model.Product, error) {
	if err := normalizeProductInput(&in); err != nil {
		return nil, err
	}
	if in.CategoryID != nil && *in.CategoryID != "" {
		if _, err := s.cats.GetByID(ctx, storeID, *in.CategoryID); err != nil {
			return nil, ErrCategoryInvalid
		}
	} else {
		in.CategoryID = nil
	}

	p := &model.Product{
		CategoryID: in.CategoryID,
		Name:       in.Name,
		SKU:        in.SKU,
		Barcode:    in.Barcode,
		BuyPrice:   in.BuyPrice,
		SellPrice:  in.SellPrice,
		Stock:      in.Stock,
		Unit:       in.Unit,
		Active:     true,
	}
	out, err := s.prods.CreateWithInitialMovement(ctx, storeID, p, actorName)
	if err != nil {
		if isPgUnique(err) || errors.Is(err, repo.ErrDuplicate) {
			return nil, ErrSkuTaken
		}
		return nil, err
	}
	return out, nil
}

func (s *CatalogService) UpdateProduct(ctx context.Context, storeID, id string, in ProductInput) (*model.Product, error) {
	if err := normalizeProductInput(&in); err != nil {
		return nil, err
	}
	if in.CategoryID != nil && *in.CategoryID != "" {
		if _, err := s.cats.GetByID(ctx, storeID, *in.CategoryID); err != nil {
			return nil, ErrCategoryInvalid
		}
	} else {
		in.CategoryID = nil
	}

	taken, err := s.prods.IsSkuTaken(ctx, storeID, in.SKU, id)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrSkuTaken
	}

	p := &model.Product{
		CategoryID: in.CategoryID,
		Name:       in.Name,
		SKU:        in.SKU,
		Barcode:    in.Barcode,
		BuyPrice:   in.BuyPrice,
		SellPrice:  in.SellPrice,
		Unit:       in.Unit,
	}
	out, err := s.prods.Update(ctx, storeID, id, p)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || isPgUnique(err) {
			if isPgUnique(err) {
				return nil, ErrSkuTaken
			}
			return nil, ErrStoreMismatch
		}
		return nil, err
	}
	return out, nil
}

func (s *CatalogService) ListProducts(ctx context.Context, storeID string, f repo.ProductFilter) (*repo.ProductPage, error) {
	return s.prods.List(ctx, storeID, f)
}

func (s *CatalogService) GetProduct(ctx context.Context, storeID, id string) (*model.Product, error) {
	return s.prods.GetByID(ctx, storeID, id)
}

func (s *CatalogService) SetProductActive(ctx context.Context, storeID, id string, active bool) error {
	err := s.prods.SetActive(ctx, storeID, id, active)
	if errors.Is(err, repo.ErrNotFound) {
		return ErrStoreMismatch
	}
	return err
}

// ── stok ─────────────────────────────────────────────────────────────

var ErrBadDirection = errors.New("arah penyesuaian tidak valid")

// AdjustStock menyesuaikan stok produk (FR-INV-002): alasan wajib (FR-INV-003),
// dilarang menghasilkan stok negatif (FR-INV-006), movement tercatat atomik.
func (s *CatalogService) AdjustStock(ctx context.Context, storeID, productID, actor, direction string, qty int64, reason string) (*model.Product, error) {
	reason = strings.TrimSpace(reason)
	productID = strings.TrimSpace(productID)
	if direction != "plus" && direction != "minus" {
		return nil, ErrBadDirection
	}
	if qty < 1 {
		return nil, fmt.Errorf("jumlah harus minimal 1")
	}
	if reason == "" {
		return nil, fmt.Errorf("alasan penyesuaian wajib diisi")
	}
	delta := int(qty)
	if direction == "minus" {
		delta = -int(qty)
	}

	p, err := s.prods.AdjustStock(ctx, storeID, productID, delta, reason, actor)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrNotFound):
			return nil, ErrStoreMismatch
		case errors.Is(err, repo.ErrStockInsufficient):
			return nil, fmt.Errorf("stok tidak boleh negatif")
		}
		return nil, err
	}
	return p, nil
}

// ListMovements riwayat pergerakan stok toko (FR-INV-003).
func (s *CatalogService) ListMovements(ctx context.Context, storeID string, f repo.MovementFilter) (*repo.MovementPage, error) {
	return s.movs.List(ctx, storeID, f)
}
