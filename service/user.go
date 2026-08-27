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
	users *repo.UserRepo
}

func NewUserService(users *repo.UserRepo) *UserService { return &UserService{users: users} }

// List mengembalikan seluruh akun dalam satu toko.
func (s *UserService) List(ctx context.Context, storeID string) ([]model.PublicUser, error) {
	users, err := s.users.ListByStore(ctx, storeID)
	if err != nil {
		return nil, err
	}
	out := make([]model.PublicUser, 0, len(users))
	for _, u := range users {
		out = append(out, u.Public())
	}
	return out, nil
}

// CreateCashier membuat akun kasir baru di toko admin.
// Kasir hanya perlu nama — tidak ada email, tidak ada password.
// Login dilakukan via switch account dari admin.
func (s *UserService) CreateCashier(ctx context.Context, storeID, name string) (*model.PublicUser, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return nil, fmt.Errorf("nama wajib diisi")
	}

	u, err := s.users.CreateCashier(ctx, storeID, name)
	if err != nil {
		return nil, err
	}
	p := u.Public()
	return &p, nil
}

// SetActive menonaktifkan/mengaktifkan akun kasir dalam toko yang sama.
// Akun admin tidak boleh diubah dari endpoint ini (FR-USR-004).
func (s *UserService) SetActive(ctx context.Context, storeID, targetUserID string, active bool) error {
	u, err := s.users.GetByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrStoreMismatch
		}
		return err
	}
	if u.StoreID != storeID {
		return ErrStoreMismatch
	}
	if u.Role != model.RoleCashier {
		return ErrNotEditable
	}
	return s.users.SetActive(ctx, targetUserID, active)
}
