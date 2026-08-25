package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshToken struct {
	UserID    string
	ExpiresAt time.Time
	Revoked   bool
}

type RefreshRepo struct {
	pool *pgxpool.Pool
}

func NewRefreshRepo(pool *pgxpool.Pool) *RefreshRepo { return &RefreshRepo{pool: pool} }

func (r *RefreshRepo) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (r *RefreshRepo) GetActiveByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	var rt RefreshToken
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, expires_at, revoked FROM refresh_tokens WHERE token_hash = $1
	`, tokenHash).Scan(&rt.UserID, &rt.ExpiresAt, &rt.Revoked)
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &rt, nil
}

func (r *RefreshRepo) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *RefreshRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1`, userID)
	return err
}
