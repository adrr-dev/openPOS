-- Fix DB after reverting cashier/user split migration (007_owner_cashier_split.sql).
-- The code is back to a unified users table; the DB still has the split schema.

-- 1. Drop the cashiers table (no data, safe to drop)
DROP TABLE IF EXISTS cashiers CASCADE;

-- 2. Add missing columns back to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS email         TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS passcode_hash TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS role          TEXT NOT NULL DEFAULT 'cashier' CHECK (role IN ('admin', 'cashier'));

-- 3. Restore unique constraint on email
ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);

-- 4. Fix transactions.cashier_id FK → users(id) instead of cashiers(id)
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_cashier_id_fkey;
ALTER TABLE transactions ADD CONSTRAINT transactions_cashier_id_fkey
    FOREIGN KEY (cashier_id) REFERENCES users(id);
