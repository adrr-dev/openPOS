package service

import (
	"context"
	"time"

	"github.com/adrr-dev/openPOS/backend/model"
	"github.com/adrr-dev/openPOS/backend/repo"
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uint) (*model.User, error)
	ListByStore(ctx context.Context, storeID uint) ([]*model.User, error)
	SetActive(ctx context.Context, id uint, active bool) error
	SetPasscode(ctx context.Context, id uint, hash *string) error
	RegisterTx(ctx context.Context, storeName, email, name, passwordHash string) (*model.User, error)
	UpdatePassword(ctx context.Context, email string, passwordHash string) error
	UpdateLastSeenAt(ctx context.Context, id uint, lastSeenAt *time.Time) error
}

type CashierRepository interface {
	GetByID(ctx context.Context, id uint) (*model.Cashier, error)
	ListByStore(ctx context.Context, storeID uint) ([]*model.Cashier, error)
	Create(ctx context.Context, storeID uint, name string) (*model.Cashier, error)
	SetActive(ctx context.Context, id uint, active bool) error
	SetPasscode(ctx context.Context, id uint, hash *string) error
	GetOrCreateByName(ctx context.Context, storeID uint, name string) (uint, error)
	Delete(ctx context.Context, id uint) error
	UpdateLastSeenAt(ctx context.Context, id uint, lastSeenAt *time.Time) error
}

type RefreshRepository interface {
	Create(ctx context.Context, userID uint, tokenHash string, expiresAt time.Time) error
	GetActiveByHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID uint) error
}

type OtpRepository interface {
	UpsertOTP(ctx context.Context, email, codeHash string, expiresAt time.Time) error
	GetOTP(ctx context.Context, email string) (*model.EmailOtp, error)
	IncrementAttempts(ctx context.Context, email string) (int, error)
	MarkVerified(ctx context.Context, email string) error
	IsEmailVerified(ctx context.Context, email string) (bool, error)
	RevokeOTP(ctx context.Context, email string) error
}

type CategoryRepository interface {
	ListByStore(ctx context.Context, storeID uint) ([]*model.Category, error)
	GetByID(ctx context.Context, storeID, id uint) (*model.Category, error)
	Create(ctx context.Context, storeID uint, name string) (*model.Category, error)
	Delete(ctx context.Context, storeID, id uint) (softDeleted bool, err error)
}

type ProductRepository interface {
	GetByID(ctx context.Context, storeID, id uint) (*model.Product, error)
	List(ctx context.Context, storeID uint, f repo.ProductFilter) (*repo.ProductPage, error)
	Create(ctx context.Context, storeID uint, p *model.Product) (*model.Product, error)
	Update(ctx context.Context, storeID, id uint, p *model.Product) (*model.Product, error)
	SetActive(ctx context.Context, storeID, id uint, active bool) error
	IsSkuTaken(ctx context.Context, storeID uint, sku string, exceptProductID uint) (bool, error)
	CreateWithInitialMovement(ctx context.Context, storeID uint, p *model.Product, actor string) (*model.Product, error)
	AdjustStock(ctx context.Context, storeID, productID uint, delta int, reason, actor string) (*model.Product, error)
	Delete(ctx context.Context, storeID, id uint) error
}

type MovementRepository interface {
	List(ctx context.Context, storeID uint, f repo.MovementFilter) (*repo.MovementPage, error)
}

type TrxRepository interface {
	Checkout(ctx context.Context, in repo.CheckoutInput) (*model.Trx, error)
	Refund(ctx context.Context, storeID, trxID uint, items map[uint]int, reason, byName string) (*model.Trx, error)
	List(ctx context.Context, storeID, cashierID uint, q, method, date string, page, limit int) ([]*model.Trx, int, error)
	GetByID(ctx context.Context, storeID, id uint) (*model.Trx, error)
}

type StoreRepository interface {
	GetSettings(ctx context.Context, storeID uint) (*model.StoreSettings, error)
	GetTimezone(ctx context.Context, storeID uint) (string, error)
	UpdateSettings(ctx context.Context, storeID uint, s *model.StoreSettings) (*model.StoreSettings, error)
	SetPasscode(ctx context.Context, storeID, userID uint, hash *string) error
}

type ReportRepository interface {
	Dashboard(ctx context.Context, storeID, cashierID uint, cashierView bool, tz string) (any, error)
	Report(ctx context.Context, storeID uint, period, tz string) (*model.ReportBundle, error)
}

type ShiftRepository interface {
	GetActiveShift(ctx context.Context, storeID, cashierID uint) (*model.CashierShift, error)
	StartShift(ctx context.Context, storeID, cashierID uint, cashierName string, openingCash int64) (*model.CashierShift, error)
	CloseShift(ctx context.Context, storeID, cashierID uint) (*model.CashierShift, error)
	ListStoreShifts(ctx context.Context, storeID uint) ([]*model.CashierShift, error)
	GetTrxStatsForShift(ctx context.Context, storeID, cashierID uint, startTime time.Time, endTime time.Time) (int64, int64, []model.TrxItem, []model.Trx, error)
}
