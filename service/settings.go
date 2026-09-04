package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/0xMinomus/openPOS/backend/model"
	"github.com/0xMinomus/openPOS/backend/repo"
)

type SettingsService struct {
	stores   *repo.StoreRepo
	users    *repo.UserRepo
	cashiers *repo.CashierRepo
	reports  *repo.ReportRepo
}

func NewSettingsService(stores *repo.StoreRepo, users *repo.UserRepo, cashiers *repo.CashierRepo, reports *repo.ReportRepo) *SettingsService {
	return &SettingsService{stores: stores, users: users, cashiers: cashiers, reports: reports}
}

func (s *SettingsService) Get(ctx context.Context, storeID uint) (*model.StoreSettings, error) {
	return s.stores.GetSettings(ctx, storeID)
}

func (s *SettingsService) Update(ctx context.Context, storeID uint, in *model.StoreSettings) (*model.StoreSettings, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("nama toko wajib diisi")
	}
	in.Timezone = strings.TrimSpace(in.Timezone)
	if in.Timezone == "" {
		in.Timezone = "Asia/Makassar"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return nil, ErrBadTimezone
	}
	if in.Paper != "58mm" && in.Paper != "80mm" {
		in.Paper = "58mm"
	}
	if in.TaxPct < 0 {
		in.TaxPct = 0
	}
	return s.stores.UpdateSettings(ctx, storeID, in)
}

// SetPasscode sets the 5-digit PIN for an owner or cashier. roleHint comes
// from the /users list entry the admin clicked ("admin"/"cashier"/"") —
// users and cashiers have independent numeric sequences, so one number can
// exist in both tables. Empty hint keeps the legacy cashier-first order.
func (s *SettingsService) SetPasscode(ctx context.Context, storeID, targetID uint, passcode, roleHint string) error {
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

	tryCashier := roleHint == "" || roleHint == string(model.RoleCashier)
	tryOwner := roleHint == "" || roleHint == string(model.RoleAdmin)

	if tryCashier {
		if c, err := s.cashiers.GetByID(ctx, targetID); err == nil && c.StoreID == storeID {
			return s.cashiers.SetPasscode(ctx, targetID, hash)
		}
	}
	if tryOwner {
		if owner, err := s.users.GetByID(ctx, targetID); err == nil && owner.StoreID == storeID {
			return s.users.SetPasscode(ctx, targetID, hash)
		}
	}

	return ErrStoreMismatch
}

func (s *SettingsService) Dashboard(ctx context.Context, storeID, cashierID uint, cashierView bool) (any, error) {
	tz, err := s.stores.GetTimezone(ctx, storeID)
	if err != nil {
		tz = "Asia/Makassar"
	}
	return s.reports.Dashboard(ctx, storeID, cashierID, cashierView, tz)
}

var (
	ErrBadTimezone = fmt.Errorf("zona waktu tidak valid")
	ErrBadPeriod   = errors.New("periode tidak valid")
)

var validPeriods = map[string]bool{"": true, "today": true, "yesterday": true, "week": true, "month": true, "all": true}

func (s *SettingsService) Report(ctx context.Context, storeID uint, period string) (*model.ReportBundle, error) {
	if !validPeriods[period] {
		return nil, ErrBadPeriod
	}
	tz, err := s.stores.GetTimezone(ctx, storeID)
	if err != nil {
		tz = "Asia/Makassar"
	}
	return s.reports.Report(ctx, storeID, period, tz)
}
