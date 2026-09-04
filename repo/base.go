package repo

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrNotFound  = errors.New("tidak ditemukan")
	ErrDuplicate = errors.New("data sudah ada")
)

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func mapDBErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	// Old backend maps Postgres 23505 -> ErrDuplicate. Keep same mapping on
	// both postgres and sqlite so responses stay identical (409 vs 400).
	if err != nil {
		msg := err.Error()
		if contains(msg, "23505") ||
			contains(msg, "duplicate key") ||
			contains(msg, "UNIQUE constraint failed") ||
			contains(msg, "unique constraint") {
			return ErrDuplicate
		}
	}
	return err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && searchStr(s, substr))
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
