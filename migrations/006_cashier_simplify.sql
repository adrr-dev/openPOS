-- Sederhanakan akun kasir: hanya perlu name (email & password dihilangkan).
-- Kasir tidak login mandiri, hanya via switch account dari admin.
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
