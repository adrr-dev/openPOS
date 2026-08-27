package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

type CashierRepo struct {
	pool *pgxpool.Pool
}

func NewCashierRepo(pool *pgxpool.Pool) *CashierRepo { return &CashierRepo{pool: pool} }

const cashierCols = `c.id, c.owner_id, c.name, c.passcode_hash, c.active, c.created_at`

func scanCashier(row pgx.Row) (*model.Cashier, error) {
	var c model.Cashier
	if err := row.Scan(&c.ID, &c.OwnerID, &c.Name, &c.PasscodeHash, &c.Active, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CashierRepo) GetByID(ctx context.Context, id string) (*model.Cashier, error) {
	return scanCashier(r.pool.QueryRow(ctx,
		`SELECT `+cashierCols+` FROM cashiers c WHERE c.id = $1`, id))
}

func (r *CashierRepo) ListByOwner(ctx context.Context, ownerID string) ([]*model.Cashier, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+cashierCols+` FROM cashiers c WHERE c.owner_id = $1 ORDER BY c.created_at ASC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*model.Cashier, 0)
	for rows.Next() {
		c, err := scanCashier(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CashierRepo) Create(ctx context.Context, ownerID, name string) (*model.Cashier, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cashiers (owner_id, name) VALUES ($1, $2) RETURNING id`,
		ownerID, name).Scan(&id)
	if err != nil {
		return nil, mapDBErr(err)
	}
	return r.GetByID(ctx, id)
}

func (r *CashierRepo) SetActive(ctx context.Context, id string, active bool) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE cashiers SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CashierRepo) SetPasscode(ctx context.Context, id string, hash *string) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE cashiers SET passcode_hash = $2 WHERE id = $1`, id, hash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
