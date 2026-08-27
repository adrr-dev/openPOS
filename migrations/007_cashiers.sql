-- 007_cashiers.sql: Separate cashiers table for sub-accounts while keeping users table for store owners (admin)

CREATE TABLE IF NOT EXISTS cashiers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id      UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    passcode_hash TEXT,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cashiers_store ON cashiers(store_id);

-- Add passcode_hash to users if not exists (for owner quick switch)
ALTER TABLE users ADD COLUMN IF NOT EXISTS passcode_hash TEXT;

-- Update transactions table foreign key to reference cashiers(id)
-- First, ensure any existing transactions have valid cashier references or drop/recreate constraint safely.
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_cashier_id_fkey;

-- If there are existing records in transactions referencing users(id) who were cashiers,
-- we should migrate them or handle FK safely. For a clean slate or migration:
-- Let's check if transactions table exists and has foreign key.
ALTER TABLE transactions ADD CONSTRAINT transactions_cashier_id_fkey
    FOREIGN KEY (cashier_id) REFERENCES cashiers(id) ON DELETE RESTRICT;
