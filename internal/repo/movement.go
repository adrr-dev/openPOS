package repo

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/internal/model"
)

type MovementFilter struct {
	Type      string // sale|refund|adjust|initial (kosong = semua)
	ProductID string
	Page      int
	Limit     int
}

type MovementPage struct {
	Items []*model.Movement `json:"items"`
	Total int               `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}

const movCols = `m.id, m.store_id, m.product_id, p.name, m.type, m.qty, m.reason, m.actor, m.created_at`

type MovementRepo struct {
	pool *pgxpool.Pool
}

func NewMovementRepo(pool *pgxpool.Pool) *MovementRepo { return &MovementRepo{pool: pool} }

func scanMovement(row interface{ Scan(...any) error }) (*model.Movement, error) {
	var m model.Movement
	if err := row.Scan(&m.ID, &m.StoreID, &m.ProductID, &m.ProductName,
		&m.Type, &m.Qty, &m.Reason, &m.Actor, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

// insertMovementTx menyimpan baris movement di dalam transaksi yang diberikan
// (dipakai bersama oleh penyesuaian stok, transaksi penjualan, dan refund).
func insertMovementTx(ctx context.Context, tx pgx.Tx, m *model.Movement) error {
	return tx.QueryRow(ctx, `
		INSERT INTO stock_movements (store_id, product_id, type, qty, reason, actor)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, m.StoreID, m.ProductID, string(m.Type), m.Qty, m.Reason, m.Actor).Scan(&m.ID, &m.CreatedAt)
}

// InsertTx menyimpan movement di dalam transaksi yang diberikan.
func (r *MovementRepo) InsertTx(ctx context.Context, tx pgx.Tx, m *model.Movement) error {
	return insertMovementTx(ctx, tx, m)
}

// List riwayat pergerakan toko, terbaru lebih dulu.
func (r *MovementRepo) List(ctx context.Context, storeID string, f MovementFilter) (*MovementPage, error) {
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	conds := []string{"m.store_id = $1"}
	args := []any{storeID}
	if f.Type != "" {
		args = append(args, f.Type)
		conds = append(conds, `m.type = $`+strconv.Itoa(len(args)))
	}
	if f.ProductID != "" {
		args = append(args, f.ProductID)
		conds = append(conds, `m.product_id = $`+strconv.Itoa(len(args)))
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM stock_movements m`+where, args...,
	).Scan(&total); err != nil {
		return nil, err
	}

	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	args = append(args, limit, (page-1)*limit)

	rows, err := r.pool.Query(ctx, `
		SELECT `+movCols+`
		FROM stock_movements m LEFT JOIN products p ON p.id = m.product_id
		`+where+`
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT $`+strconv.Itoa(limitIdx)+` OFFSET $`+strconv.Itoa(offsetIdx), args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*model.Movement, 0, limit)
	for rows.Next() {
		m, err := scanMovement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &MovementPage{Items: items, Total: total, Page: page, Limit: limit}, nil
}
