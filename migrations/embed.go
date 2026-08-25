// Package migrations menampung file SQL migrasi skema database.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
