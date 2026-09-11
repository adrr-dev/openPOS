package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/adrr-dev/openPOS/backend/model"
)

type SettingsService struct {
	stores   StoreRepository
	users    UserRepository
	cashiers CashierRepository
	reports  ReportRepository
}

func NewSettingsService(stores StoreRepository, users UserRepository, cashiers CashierRepository, reports ReportRepository) *SettingsService {
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
	if in.TaxPct > 100 {
		return nil, ErrBadTaxPct
	}
	if err := validateExtendedSettings(in); err != nil {
		return nil, err
	}
	return s.stores.UpdateSettings(ctx, storeID, in)
}

var validBusinessTypes = map[string]bool{
	"": true, "retail": true, "fnb": true, "fashion": true, "jasa": true, "lainnya": true,
}

// Label days jam operasional (grup tampilan frontend) — tepat 3 entri.
var validHoursDays = []string{"Senin – Jumat", "Sabtu", "Minggu"}

func validateExtendedSettings(in *model.StoreSettings) error {
	in.BusinessType = strings.TrimSpace(in.BusinessType)
	if !validBusinessTypes[in.BusinessType] {
		return ErrBadBusinessType
	}
	in.Email = strings.TrimSpace(in.Email)
	if in.Email != "" && !isEmail(in.Email) {
		return ErrBadStoreEmail
	}
	in.City = strings.TrimSpace(in.City)
	in.Province = strings.TrimSpace(in.Province)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "IDR"
	}
	if len(in.Currency) != 3 {
		return ErrBadCurrency
	}
	for _, ch := range in.Currency {
		if ch < 'A' || ch > 'Z' {
			return ErrBadCurrency
		}
	}
	if in.Hours == nil {
		in.Hours = []model.StoreHours{}
	}
	if len(in.Hours) != 0 && len(in.Hours) != 3 {
		return ErrBadHours
	}
	for i, h := range in.Hours {
		if h.Days != validHoursDays[i] {
			return ErrBadHours
		}
		if h.Open == nil && h.Close == nil {
			continue
		}
		if h.Open == nil || h.Close == nil {
			return ErrBadHours
		}
		open, close := strings.TrimSpace(*h.Open), strings.TrimSpace(*h.Close)
		if !validHourMinute(open) || !validHourMinute(close) || open >= close {
			return ErrBadHours
		}
		in.Hours[i].Open, in.Hours[i].Close = &open, &close
	}
	if len(in.ReceiptFooter) > 200 {
		return ErrBadReceiptFooter
	}
	if len(in.TaxName) > 20 {
		return ErrBadTaxName
	}
	in.TaxName = strings.TrimSpace(in.TaxName)
	switch in.TaxRounding {
	case "", "none":
		in.TaxRounding = "none"
	case "down", "up":
	default:
		return ErrBadTaxRounding
	}
	if in.TaxApplyTo == "" {
		in.TaxApplyTo = "all"
	}
	if in.TaxApplyTo != "all" {
		return ErrBadTaxApplyTo
	}
	return nil
}

func validHourMinute(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	for _, idx := range []int{0, 1, 3, 4} {
		if s[idx] < '0' || s[idx] > '9' {
			return false
		}
	}
	hh := int(s[0]-'0')*10 + int(s[1]-'0')
	mm := int(s[3]-'0')*10 + int(s[4]-'0')
	return hh < 24 && mm < 60
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
	ErrBadTimezone      = fmt.Errorf("zona waktu tidak valid")
	ErrBadPeriod        = errors.New("periode tidak valid")
	ErrBadBusinessType  = errors.New("Jenis usaha tidak valid.")
	ErrBadStoreEmail    = errors.New("Email tidak valid.")
	ErrBadCurrency      = errors.New("Kode mata uang tidak valid.")
	ErrBadHours         = errors.New("Jam operasional tidak valid.")
	ErrBadReceiptFooter = errors.New("Pesan footer maksimal 200 karakter.")
	ErrBadTaxName       = errors.New("Nama pajak maksimal 20 karakter.")
	ErrBadTaxPct        = errors.New("Tarif pajak maksimal 100 persen.")
	ErrBadTaxRounding   = errors.New("Pembulatan pajak tidak valid.")
	ErrBadTaxApplyTo    = errors.New("Cakupan pajak tidak valid.")
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
