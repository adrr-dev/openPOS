package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/0xMinomus/openPOS/backend/model"
	"github.com/0xMinomus/openPOS/backend/repo"
)

var (
	ErrStoreMismatch = errors.New("akun tidak ditemukan di toko Anda")
	ErrNotEditable   = errors.New("hanya akun kasir yang dapat dinonaktifkan")
)

type UserService struct {
	users    *repo.UserRepo
	cashiers *repo.CashierRepo
}

func NewUserService(users *repo.UserRepo, cashiers *repo.CashierRepo) *UserService {
	return &UserService{users: users, cashiers: cashiers}
}

// ListCashiers mengembalikan seluruh kasir dalam satu toko.
func (s *UserService) ListCashiers(ctx context.Context, ownerID string) ([]*model.Cashier, error) {
	return s.cashiers.ListByOwner(ctx, ownerID)
}

// CreateCashier membuat kasir baru di toko owner.
func (s *UserService) CreateCashier(ctx context.Context, ownerID, name string) (*model.Cashier, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("nama wajib diisi")
	}
	return s.cashiers.Create(ctx, ownerID, name)
}

// SetActive menonaktifkan/mengaktifkan kasir milik owner tertentu.
func (s *UserService) SetActive(ctx context.Context, ownerID, cashierID string, active bool) error {
	c, err := s.cashiers.GetByID(ctx, cashierID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrStoreMismatch
		}
		return err
	}
	if c.OwnerID != ownerID {
		return ErrStoreMismatch
	}
	return s.cashiers.SetActive(ctx, cashierID, active)
}
