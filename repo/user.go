package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

var (
	ErrNotFound  = errors.New("tidak ditemukan")
	ErrDuplicate = errors.New("data sudah ada")
)

func mapDBErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

// userCols — users sekarang HANYA punya: id, store_id, name, email, password_hash, active, created_at
const userCols = `u.id, u.store_id, u.name, u.email, u.password_hash, u.active, u.created_at, s.name`

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	if err := row.Scan(&u.ID, &u.StoreID, &u.Name, &u.Email, &u.PasswordHash,
		&u.Active, &u.CreatedAt, &u.StoreName); err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByEmail mengambil owner berdasarkan email (untuk login).
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users u JOIN stores s ON s.id = u.store_id WHERE u.email = $1`, email))
	if err != nil {
		return nil, mapDBErr(err)
	}
	return u, nil
}

// GetByID mengambil owner berdasarkan ID.
func (r *UserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users u JOIN stores s ON s.id = u.store_id WHERE u.id = $1`, id))
	if err != nil {
		return nil, mapDBErr(err)
	}
	return u, nil
}

// ListByStore mengembalikan owner dari toko tertentu (hanya 1).
func (r *UserRepo) ListByStore(ctx context.Context, storeID string) ([]*model.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+userCols+` FROM users u JOIN stores s ON s.id = u.store_id
		WHERE u.store_id = $1 ORDER BY u.created_at ASC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// RegisterTx membuat store + owner sekaligus dalam satu transaksi.
func (r *UserRepo) RegisterTx(ctx context.Context, storeName, email, name, passwordHash string) (*model.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var storeID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO stores (name) VALUES ($1) RETURNING id`, storeName,
	).Scan(&storeID); err != nil {
		return nil, mapDBErr(err)
	}

	u := &model.User{
		ID:           uuid.NewString(),
		StoreID:      storeID.String(),
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, store_id, email, name, password_hash)
		VALUES ($1, $2, $3, $4, $5)
	`, u.ID, u.StoreID, u.Email, u.Name, u.PasswordHash); err != nil {
		return nil, mapDBErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	full, err := r.GetByID(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return full, nil
}
