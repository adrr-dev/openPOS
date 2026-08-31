package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

type OtpRepo struct {
	pool *pgxpool.Pool
}

func NewOtpRepo(pool *pgxpool.Pool) *OtpRepo {
	return &OtpRepo{pool: pool}
}

func (r *OtpRepo) UpsertOTP(ctx context.Context, email, codeHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO email_otps (email, code_hash, expires_at, attempts, last_sent_at, verified_at, created_at)
		VALUES ($1, $2, $3, 0, now(), NULL, now())
		ON CONFLICT (email) DO UPDATE SET
			code_hash = EXCLUDED.code_hash,
			expires_at = EXCLUDED.expires_at,
			attempts = 0,
			last_sent_at = now(),
			verified_at = NULL,
			created_at = now()
	`, email, codeHash, expiresAt)
	return err
}

func (r *OtpRepo) GetOTP(ctx context.Context, email string) (*model.EmailOtp, error) {
	var o model.EmailOtp
	var verifiedAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT email, code_hash, expires_at, attempts, last_sent_at, verified_at, created_at
		FROM email_otps WHERE email = $1
	`, email).Scan(&o.Email, &o.CodeHash, &o.ExpiresAt, &o.Attempts, &o.LastSentAt, &verifiedAt, &o.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	o.VerifiedAt = verifiedAt
	return &o, nil
}

func (r *OtpRepo) IncrementAttempts(ctx context.Context, email string) (int, error) {
	var attempts int
	err := r.pool.QueryRow(ctx, `
		UPDATE email_otps SET attempts = attempts + 1 WHERE email = $1 RETURNING attempts
	`, email).Scan(&attempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return attempts, nil
}

func (r *OtpRepo) MarkVerified(ctx context.Context, email string) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE email_otps SET verified_at = now() WHERE email = $1
	`, email)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *OtpRepo) IsEmailVerified(ctx context.Context, email string) (bool, error) {
	var verified bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM email_otps WHERE email = $1 AND verified_at IS NOT NULL
		)
	`, email).Scan(&verified)
	return verified, err
}

func (r *OtpRepo) RevokeOTP(ctx context.Context, email string) error {
	_, err := r.pool.Exec(ctx, `UPDATE email_otps SET attempts = 3, verified_at = NULL WHERE email = $1`, email)
	return err
}
