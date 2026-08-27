package model

import "time"

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleCashier Role = "cashier"
)

type Store struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID           string    `json:"id"`
	StoreID      string    `json:"store_id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	PasscodeHash *string   `json:"-"`
	Role         Role      `json:"role"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	StoreName    string    `json:"store_name,omitempty"`
}

type Cashier struct {
	ID           string    `json:"id"`
	StoreID      string    `json:"store_id"`
	Name         string    `json:"name"`
	PasscodeHash *string   `json:"-"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
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

// PublicUser adalah bentuk user yang dikirim ke klien (tanpa hash).
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
