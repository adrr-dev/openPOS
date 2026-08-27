-- Owner + Cashier split:
-- users = hanya owner (admin toko)
-- cashiers = sub-accounts (kasir)

-- 1. Buat tabel cashiers
CREATE TABLE IF NOT EXISTS cashiers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    passcode_hash TEXT,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_cashiers_owner ON cashiers(owner_id);

-- 2. Tambah kolom sementara untuk mapping old user ID
ALTER TABLE cashiers ADD COLUMN old_user_id UUID;

-- 3. Pindahkan semua cashier dari users ke cashiers
INSERT INTO cashiers (owner_id, name, passcode_hash, active, created_at, old_user_id)
SELECT
    u2.id AS owner_id,
    u.name,
    u.passcode_hash,
    u.active,
    u.created_at,
    u.id AS old_user_id
FROM users u
JOIN users u2 ON u2.store_id = u.store_id AND u2.role = 'admin'
WHERE u.role = 'cashier';

-- 4. Drop NOT NULL dari transactions.cashier_id dulu (FK masih指向 users)
ALTER TABLE transactions ALTER COLUMN cashier_id DROP NOT NULL;

-- 5. Update transactions.cashier_id → cashiers.id
UPDATE transactions t
SET cashier_id = c.id
FROM cashiers c
WHERE t.cashier_id = c.old_user_id;

-- 6. Sekarang aman drop old cashier dari users
DELETE FROM users WHERE id IN (SELECT old_user_id FROM cashiers);

-- 7. Drop kolom sementara
ALTER TABLE cashiers DROP COLUMN old_user_id;

-- 8. Drop kolom yang tidak perlu dari users
ALTER TABLE users DROP COLUMN IF EXISTS role;
ALTER TABLE users DROP COLUMN IF EXISTS email;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users DROP COLUMN IF EXISTS passcode_hash;

-- 9. Update FK transactions.cashier_id → cashiers(id)
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_cashier_id_fkey;
ALTER TABLE transactions ALTER COLUMN cashier_id SET NOT NULL;
ALTER TABLE transactions
    ADD CONSTRAINT transactions_cashier_id_fkey
    FOREIGN KEY (cashier_id) REFERENCES cashiers(id);
