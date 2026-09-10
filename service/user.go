package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/adrr-dev/openPOS/backend/model"
	"github.com/adrr-dev/openPOS/backend/repo"
)

var (
	ErrStoreMismatch = errors.New("akun tidak ditemukan di toko Anda")
	ErrNotEditable   = errors.New("hanya akun kasir yang dapat dinonaktifkan")
	ErrNotRenamable  = errors.New("hanya akun kasir yang dapat diubah")
)

type UserService struct {
	users    UserRepository
	cashiers CashierRepository
}

func NewUserService(users UserRepository, cashiers CashierRepository) *UserService {
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

func (s *UserService) Delete(ctx context.Context, storeID, targetID uint) error {
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
	return s.cashiers.Delete(ctx, targetID)
}

func (s *UserService) Rename(ctx context.Context, storeID, targetID uint, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("Nama kasir wajib diisi.")
	}
	c, err := s.cashiers.GetByID(ctx, targetID)
	if err == nil {
		if c.StoreID != storeID {
			return ErrStoreMismatch
		}
		return s.cashiers.UpdateName(ctx, targetID, name)
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return err
	}
	// Cashier not found: check if ID belongs to admin user in same store (ID collision case MEMORY.md §10)
	u, uErr := s.users.GetByID(ctx, targetID)
	if uErr == nil && u.StoreID == storeID {
		return ErrNotRenamable
	}
	return ErrStoreMismatch
}
