package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

type StoreRepo struct {
	db *gorm.DB
}

func NewStoreRepo(db *gorm.DB) *StoreRepo { return &StoreRepo{db: db} }

func (r *StoreRepo) GetSettings(ctx context.Context, storeID uint) (*model.StoreSettings, error) {
	var st model.Store
	if err := r.db.WithContext(ctx).Where("id = ?", storeID).First(&st).Error; err != nil {
		return nil, mapDBErr(err)
	}
	return &model.StoreSettings{
		Name:          st.Name,
		Address:       st.Address,
		Phone:         st.Phone,
		TaxEnabled:    st.TaxEnabled,
		TaxPct:        st.TaxPct,
		ReceiptHeader: st.ReceiptHeader,
		ReceiptFooter: st.ReceiptFooter,
		Paper:         st.Paper,
		Timezone:      st.Timezone,
	}, nil
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
		"name":           s.Name,
		"address":        s.Address,
		"phone":          s.Phone,
		"tax_enabled":    s.TaxEnabled,
		"tax_pct":        s.TaxPct,
		"receipt_header": s.ReceiptHeader,
		"receipt_footer": s.ReceiptFooter,
		"paper":          s.Paper,
		"timezone":       s.Timezone,
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
