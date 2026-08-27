package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

type StoreRepo struct {
	pool *pgxpool.Pool
}

func NewStoreRepo(pool *pgxpool.Pool) *StoreRepo { return &StoreRepo{pool: pool} }

const setCols = `name, address, phone, tax_enabled, tax_pct,
	receipt_header, receipt_footer, paper, timezone`

func scanSettings(row interface{ Scan(...any) error }) (*model.StoreSettings, error) {
	var s model.StoreSettings
	if err := row.Scan(&s.Name, &s.Address, &s.Phone, &s.TaxEnabled, &s.TaxPct,
		&s.ReceiptHeader, &s.ReceiptFooter, &s.Paper, &s.Timezone); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *StoreRepo) GetSettings(ctx context.Context, storeID string) (*model.StoreSettings, error) {
	return scanSettings(r.pool.QueryRow(ctx, `SELECT `+setCols+` FROM stores WHERE id = $1`, storeID))
}

// GetTimezone mengembalikan zona waktu toko (fallback Asia/Makassar bila kosong).
func (r *StoreRepo) GetTimezone(ctx context.Context, storeID string) (string, error) {
	var tz string
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(NULLIF(timezone,''),'Asia/Makassar') FROM stores WHERE id = $1`, storeID).Scan(&tz)
	return tz, err
}

func (r *StoreRepo) UpdateSettings(ctx context.Context, storeID string, s *model.StoreSettings) (*model.StoreSettings, error) {
	return scanSettings(r.pool.QueryRow(ctx, `
		UPDATE stores SET name=$2, address=$3, phone=$4, tax_enabled=$5, tax_pct=$6,
			receipt_header=$7, receipt_footer=$8, paper=$9, timezone=$10
		WHERE id = $1 RETURNING `+setCols+`
	`, storeID, s.Name, s.Address, s.Phone, s.TaxEnabled, s.TaxPct,
		s.ReceiptHeader, s.ReceiptFooter, s.Paper, s.Timezone))
}
