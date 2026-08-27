package model

import "time"

type Store struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// User adalah pemilik toko (admin). Hanya 1 owner per toko.
type User struct {
	ID           string    `json:"id"`
	StoreID      string    `json:"store_id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	StoreName    string    `json:"store_name,omitempty"`
}

// Cashier adalah sub-account kasir. Dibuat oleh owner.
type Cashier struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"-"`
	Name         string    `json:"name"`
	PasscodeHash *string   `json:"-"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

type PublicUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email,omitempty"`
	Name      string    `json:"name"`
	StoreID   string    `json:"store_id"`
	StoreName string    `json:"store_name,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

func (u *User) Public() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		StoreID:   u.StoreID,
		StoreName: u.StoreName,
		CreatedAt: u.CreatedAt,
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
