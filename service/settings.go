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

func (s *SettingsService) Get(ctx context.Context, storeID string) (*model.StoreSettings, error) {
	return s.stores.GetSettings(ctx, storeID)
}

func (s *SettingsService) Update(ctx context.Context, storeID string, in *model.StoreSettings) (*model.StoreSettings, error) {
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

// SetPasscode: hash 5 digit; string kosong = hapus passcode.
// Admin dapat mengatur passcode kasir mana pun dalam tokonya.
func (s *SettingsService) SetPasscode(ctx context.Context, storeID, cashierID, passcode string) error {
	c, err := s.cashiers.GetByID(ctx, cashierID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrStoreMismatch
		}
		return err
	}
	// pastikan kasir ini milik owner di toko yang benar
	owner, err := s.users.GetByID(ctx, c.OwnerID)
	if err != nil || owner.StoreID != storeID {
		return ErrStoreMismatch
	}
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
	err = s.cashiers.SetPasscode(ctx, cashierID, hash)
	if err == repo.ErrNotFound {
		return ErrStoreMismatch
	}
	return err
}

func (s *SettingsService) Dashboard(ctx context.Context, storeID, callerID string, actingAsCashierID *string, cashierView bool) (any, error) {
	tz, err := s.stores.GetTimezone(ctx, storeID)
	if err != nil {
		tz = "Asia/Makassar"
	}
	// cashierID: gunakan acting-as cashier bila ada, else caller ID (owner)
	cashierID := callerID
	if actingAsCashierID != nil {
		cashierID = *actingAsCashierID
	}
	return s.reports.Dashboard(ctx, storeID, cashierID, cashierView, tz)
}

var (
	ErrBadTimezone = fmt.Errorf("zona waktu tidak valid")
	ErrBadPeriod   = errors.New("periode tidak valid")
)

var validPeriods = map[string]bool{"": true, "today": true, "yesterday": true, "week": true, "month": true, "all": true}

func (s *SettingsService) Report(ctx context.Context, storeID, period string) (*model.ReportBundle, error) {
	if !validPeriods[period] {
		return nil, ErrBadPeriod
	}
	tz, err := s.stores.GetTimezone(ctx, storeID)
	if err != nil {
		tz = "Asia/Makassar"
	}
	return s.reports.Report(ctx, storeID, period, tz)
}
