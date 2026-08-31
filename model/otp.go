package model

import "time"

type EmailOtp struct {
    Email       string     `json:"email"`
    CodeHash    string     `json:"-"`
    ExpiresAt   time.Time  `json:"expires_at"`
    Attempts    int        `json:"attempts"`
    LastSentAt  time.Time  `json:"last_sent_at"`
    VerifiedAt  *time.Time `json:"verified_at,omitempty"`
    CreatedAt   time.Time  `json:"created_at"`
}
