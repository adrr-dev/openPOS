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

func (s *UserService) List(ctx context.Context, storeID uint) ([]model.PublicUser, error) {
	owners, err := s.users.ListByStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	cashiers, err := s.cashiers.ListByStore(ctx, storeID)
	if err != nil {
		return nil, err
	}

	out := make([]model.PublicUser, 0, len(owners)+len(cashiers))
	for _, u := range owners {
		out = append(out, u.Public())
	}
	storeName := ""
	if len(owners) > 0 {
		storeName = owners[0].StoreName
	}
	for _, c := range cashiers {
		out = append(out, c.Public(storeName))
	}
	return out, nil
}

func (s *UserService) CreateCashier(ctx context.Context, storeID uint, name string) (*model.PublicUser, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("nama wajib diisi")
	}

	c, err := s.cashiers.Create(ctx, storeID, name)
	if err != nil {
		return nil, err
	}
	owners, _ := s.users.ListByStore(ctx, storeID)
	storeName := ""
	if len(owners) > 0 {
		storeName = owners[0].StoreName
	}
	pub := c.Public(storeName)
	return &pub, nil
}

func (s *UserService) SetActive(ctx context.Context, storeID, targetID uint, active bool) error {
	c, err := s.cashiers.GetByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrStoreMismatch
		}
		return err
	}
	if c.StoreID != storeID {
		return ErrStoreMismatch
	}
	return s.cashiers.SetActive(ctx, targetID, active)
}
