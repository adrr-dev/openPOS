package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

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

// List mengembalikan seluruh akun dalam satu toko (Admin + Cashiers).
func (s *UserService) List(ctx context.Context, storeID string) ([]model.PublicUser, error) {
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

// CreateCashier membuat akun kasir baru di toko admin (hanya butuh nama).
func (s *UserService) CreateCashier(ctx context.Context, storeID, name string) (*model.PublicUser, error) {
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

// SetActive menonaktifkan/mengaktifkan akun kasir dalam toko yang sama.
func (s *UserService) SetActive(ctx context.Context, storeID, targetID string, active bool) error {
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

// SetPasscode mengatur passcode untuk admin atau kasir.
func (s *UserService) SetPasscode(ctx context.Context, storeID, targetID, passcode string) error {
	passcode = strings.TrimSpace(passcode)
	var hash *string
	if passcode != "" {
		if len(passcode) != 5 {
			return fmt.Errorf("passcode harus 5 angka")
		}
		for _, ch := range passcode {
			if ch < '0' || ch > '9' {
				return fmt.Errorf("passcode harus 5 angka")
			}
		}
		h, err := bcrypt.GenerateFromPassword([]byte(passcode), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hs := string(h)
		hash = &hs
	}

	// Cek apakah target adalah kasir
	c, err := s.cashiers.GetByID(ctx, targetID)
	if err == nil && c.StoreID == storeID {
		return s.cashiers.SetPasscode(ctx, targetID, hash)
	}

	// Cek apakah target adalah admin (owner)
	owner, err := s.users.GetByID(ctx, targetID)
	if err == nil && owner.StoreID == storeID {
		return s.users.SetPasscode(ctx, targetID, hash)
	}

	return ErrStoreMismatch
}
