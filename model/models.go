package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleCashier Role = "cashier"
)

type Store struct {
	ID            string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Name          string    `gorm:"not null" json:"name"`
	Address       string    `gorm:"not null;default:''" json:"address"`
	Phone         string    `gorm:"not null;default:''" json:"phone"`
	TaxEnabled    bool      `gorm:"not null;default:false" json:"taxEnabled"`
	TaxPct        float64   `gorm:"not null;default:0" json:"taxPct"`
	ReceiptHeader string    `gorm:"not null;default:'Terima kasih sudah berbelanja'" json:"receiptHeader"`
	ReceiptFooter string    `gorm:"not null;default:'Barang yang sudah dibeli tidak dapat ditukar'" json:"receiptFooter"`
	Paper         string    `gorm:"not null;default:'58mm'" json:"paper"`
	Timezone      string    `gorm:"not null;default:'Asia/Makassar'" json:"timezone"`
	CreatedAt     time.Time `gorm:"not null" json:"created_at"`
}

func (Store) TableName() string {
	return "stores"
}

func (s *Store) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	return nil
}

type User struct {
	ID              string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	StoreID         string     `gorm:"type:varchar(36);not null;index" json:"store_id"`
	Email           string     `gorm:"not null;unique" json:"email"`
	Name            string     `gorm:"not null" json:"name"`
	PasswordHash    string     `gorm:"not null" json:"-"`
	PasscodeHash    *string    `json:"-"`
	Role            Role       `gorm:"not null;default:'cashier'" json:"role"`
	Active          bool       `gorm:"not null;default:true" json:"active"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `gorm:"not null" json:"created_at"`
	StoreName       string     `gorm:"-" json:"store_name,omitempty"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	return nil
}

type Cashier struct {
	ID           string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	StoreID      string    `gorm:"type:varchar(36);not null;index" json:"store_id"`
	Name         string    `gorm:"not null" json:"name"`
	PasscodeHash *string   `json:"-"`
	Active       bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt    time.Time `gorm:"not null" json:"created_at"`
}

func (Cashier) TableName() string {
	return "cashiers"
}

func (c *Cashier) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	return nil
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
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	Active    bool      `json:"active"`
	StoreID   string    `json:"store_id"`
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
	ID        string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	UserID    string    `gorm:"type:varchar(36);not null;index" json:"user_id"`
	TokenHash string    `gorm:"not null;unique" json:"token_hash"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	Revoked   bool      `gorm:"not null;default:false" json:"revoked"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

func (rt *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if rt.ID == "" {
		rt.ID = uuid.NewString()
	}
	if rt.CreatedAt.IsZero() {
		rt.CreatedAt = time.Now()
	}
	return nil
}
