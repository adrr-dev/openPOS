package repo

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"github.com/adrr-dev/openPOS/backend/model"
)

func decodeStoreHours(raw string) []model.StoreHours {
	if raw == "" {
		return []model.StoreHours{}
	}
	var h []model.StoreHours
	if err := json.Unmarshal([]byte(raw), &h); err != nil || h == nil {
		return []model.StoreHours{}
	}
	return h
}

func encodeStoreHours(h []model.StoreHours) string {
	if h == nil {
		h = []model.StoreHours{}
	}
	buf, err := json.Marshal(h)
	if err != nil {
		return "[]"
	}
	return string(buf)
}

type StoreRepo struct {
	db *gorm.DB
}

func NewStoreRepo(db *gorm.DB) *StoreRepo { return &StoreRepo{db: db} }

func (r *StoreRepo) GetSettings(ctx context.Context, storeID uint) (*model.StoreSettings, error) {
	var st model.Store
	if err := r.db.WithContext(ctx).Where("id = ?", storeID).First(&st).Error; err != nil {
		return nil, mapDBErr(err)
	}
	s := &model.StoreSettings{
		Name:                st.Name,
		Address:             st.Address,
		Phone:               st.Phone,
		TaxEnabled:          st.TaxEnabled,
		TaxPct:              st.TaxPct,
		ReceiptHeader:       st.ReceiptHeader,
		ReceiptFooter:       st.ReceiptFooter,
		Paper:               st.Paper,
		Timezone:            st.Timezone,
		BusinessType:        st.BusinessType,
		Email:               st.Email,
		City:                st.City,
		Province:            st.Province,
		Currency:            st.Currency,
		Hours:               decodeStoreHours(st.Hours),
		ReceiptShowLogo:     st.ReceiptShowLogo,
		ReceiptShowCashier:  st.ReceiptShowCashier,
		ReceiptShowMethod:   st.ReceiptShowMethod,
		ReceiptShowTax:      st.ReceiptShowTax,
		ReceiptShowDiscount: st.ReceiptShowDiscount,
		ReceiptShowNote:     st.ReceiptShowNote,
		TaxName:             st.TaxName,
		TaxInclusive:        st.TaxInclusive,
		TaxRounding:         st.TaxRounding,
		TaxApplyTo:          st.TaxApplyTo,
	}
	// Normalisasi default aman untuk baris lama / nilai kosong.
	if s.Currency == "" {
		s.Currency = "IDR"
	}
	if s.TaxRounding == "" {
		s.TaxRounding = "none"
	}
	if s.TaxApplyTo == "" {
		s.TaxApplyTo = "all"
	}
	return s, nil
}

func (r *StoreRepo) GetTimezone(ctx context.Context, storeID uint) (string, error) {
	var st model.Store
	if err := r.db.WithContext(ctx).Where("id = ?", storeID).First(&st).Error; err != nil {
		return "Asia/Makassar", mapDBErr(err)
	}
	if st.Timezone == "" {
		return "Asia/Makassar", nil
	}
	return st.Timezone, nil
}

func (r *StoreRepo) UpdateSettings(ctx context.Context, storeID uint, s *model.StoreSettings) (*model.StoreSettings, error) {
	res := r.db.WithContext(ctx).Model(&model.Store{}).Where("id = ?", storeID).Updates(map[string]interface{}{
		"name":                  s.Name,
		"address":               s.Address,
		"phone":                 s.Phone,
		"tax_enabled":           s.TaxEnabled,
		"tax_pct":               s.TaxPct,
		"receipt_header":        s.ReceiptHeader,
		"receipt_footer":        s.ReceiptFooter,
		"paper":                 s.Paper,
		"timezone":              s.Timezone,
		"business_type":         s.BusinessType,
		"email":                 s.Email,
		"city":                  s.City,
		"province":              s.Province,
		"currency":              s.Currency,
		"hours":                 encodeStoreHours(s.Hours),
		"receipt_show_logo":     s.ReceiptShowLogo,
		"receipt_show_cashier":  s.ReceiptShowCashier,
		"receipt_show_method":   s.ReceiptShowMethod,
		"receipt_show_tax":      s.ReceiptShowTax,
		"receipt_show_discount": s.ReceiptShowDiscount,
		"receipt_show_note":     s.ReceiptShowNote,
		"tax_name":              s.TaxName,
		"tax_inclusive":         s.TaxInclusive,
		"tax_rounding":          s.TaxRounding,
		"tax_apply_to":          s.TaxApplyTo,
	})
	if res.Error != nil {
		return nil, mapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.GetSettings(ctx, storeID)
}

func (r *StoreRepo) SetPasscode(ctx context.Context, storeID, userID uint, hash *string) error {
	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ? AND store_id = ?", userID, storeID).Update("passcode_hash", hash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
