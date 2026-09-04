package model

import (
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleCashier Role = "cashier"
)

// Model is gorm.Model with the API's JSON names. gorm.Model itself carries
// no JSON tags and would serialize as "ID"/"CreatedAt", breaking every
// client expecting "id". Behavior is identical: auto-increment uint PK,
// auto timestamps, soft delete via DeletedAt (hidden from JSON).
type Model struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Store struct {
	Model
	Name          string  `gorm:"not null" json:"name"`
	Address       string  `gorm:"not null;default:''" json:"address"`
	Phone         string  `gorm:"not null;default:''" json:"phone"`
	TaxEnabled    bool    `gorm:"not null;default:false" json:"taxEnabled"`
	TaxPct        float64 `gorm:"not null;default:0" json:"taxPct"`
	ReceiptHeader string  `gorm:"not null;default:'Terima kasih sudah berbelanja'" json:"receiptHeader"`
	ReceiptFooter string  `gorm:"not null;default:'Barang yang sudah dibeli tidak dapat ditukar'" json:"receiptFooter"`
	Paper         string  `gorm:"not null;default:'58mm'" json:"paper"`
	Timezone      string  `gorm:"not null;default:'Asia/Makassar'" json:"timezone"`
}

func (Store) TableName() string {
	return "stores"
}

type User struct {
	Model
	StoreID         uint       `gorm:"not null;index" json:"store_id"`
	Email           string     `gorm:"not null;unique" json:"email"`
	Name            string     `gorm:"not null" json:"name"`
	PasswordHash    string     `gorm:"not null" json:"-"`
	PasscodeHash    *string    `json:"-"`
	Role            Role       `gorm:"not null;default:'cashier'" json:"role"`
	Active          bool       `gorm:"not null;default:true" json:"active"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	StoreName       string     `gorm:"-" json:"store_name,omitempty"`
}

func (User) TableName() string {
	return "users"
}

type Cashier struct {
	Model
	StoreID      uint    `gorm:"not null;index" json:"store_id"`
	Name         string  `gorm:"not null" json:"name"`
	PasscodeHash *string `json:"-"`
	Active       bool    `gorm:"not null;default:true" json:"active"`
}

func (Cashier) TableName() string {
	return "cashiers"
}

func (c *Cashier) Public(storeName string) PublicUser {
	return PublicUser{
		ID:        c.ID,
		Email:     "",
		Name:      c.Name,
		Role:      RoleCashier,
		Active:    c.Active,
		StoreID:   c.StoreID,
		StoreName: storeName,
		CreatedAt: c.CreatedAt,
	}
}

type PublicUser struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	Active    bool      `json:"active"`
	StoreID   uint      `json:"store_id"`
	StoreName string    `json:"store_name,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

func (u *User) Public() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		Active:    u.Active,
		StoreID:   u.StoreID,
		StoreName: u.StoreName,
		CreatedAt: u.CreatedAt,
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshToken struct {
	Model
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	TokenHash string    `gorm:"not null;unique" json:"token_hash"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	Revoked   bool      `gorm:"not null;default:false" json:"revoked"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
