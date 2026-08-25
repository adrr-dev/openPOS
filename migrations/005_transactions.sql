-- Transaksi + item snapshot + refund (FR-POS-009..011, FR-REF-001..006, EC-001/004/008)

-- Pengaturan toko dipindah ke DB (dibutuhkan perhitungan pajak checkout);
-- endpoint settings lengkap menyusul di modul #6.
ALTER TABLE stores ADD COLUMN IF NOT EXISTS address        TEXT NOT NULL DEFAULT '';
ALTER TABLE stores ADD COLUMN IF NOT EXISTS phone         TEXT NOT NULL DEFAULT '';
ALTER TABLE stores ADD COLUMN IF NOT EXISTS tax_enabled   BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE stores ADD COLUMN IF NOT EXISTS tax_pct       DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE stores ADD COLUMN IF NOT EXISTS receipt_header TEXT NOT NULL DEFAULT 'Terima kasih sudah berbelanja';
ALTER TABLE stores ADD COLUMN IF NOT EXISTS receipt_footer TEXT NOT NULL DEFAULT 'Barang yang sudah dibeli tidak dapat ditukar';
ALTER TABLE stores ADD COLUMN IF NOT EXISTS paper         TEXT NOT NULL DEFAULT '58mm';
ALTER TABLE stores ADD COLUMN IF NOT EXISTS timezone      TEXT NOT NULL DEFAULT 'Asia/Makassar';

CREATE TABLE IF NOT EXISTS transactions (
    id           TEXT PRIMARY KEY, -- 'TRX-0001' unik per toko via (store_id, seq)
    seq          INTEGER NOT NULL,
    store_id     UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    cashier_id   UUID NOT NULL REFERENCES users(id),
    cashier_name TEXT NOT NULL,
    subtotal     BIGINT NOT NULL CHECK (subtotal >= 0),
    discount     BIGINT NOT NULL DEFAULT 0 CHECK (discount >= 0),
    tax          BIGINT NOT NULL DEFAULT 0 CHECK (tax >= 0),
    total        BIGINT NOT NULL CHECK (total >= 0),
    method       TEXT NOT NULL CHECK (method IN ('Cash', 'Bank Transfer', 'QRIS', 'E-Wallet', 'Card')),
    paid         BIGINT NOT NULL CHECK (paid >= 0),
    change       BIGINT NOT NULL DEFAULT 0 CHECK (change >= 0),
    status       TEXT NOT NULL DEFAULT 'completed'
                 CHECK (status IN ('pending', 'completed', 'cancelled', 'refunded')),
    customer     TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_trx_store_time ON transactions (store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_trx_cashier ON transactions (cashier_id);

-- Snapshot nama & harga saat transaksi (EC-008): histori tak bergantung data produk kini
CREATE TABLE IF NOT EXISTS transaction_items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trx_id     TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id),
    name       TEXT NOT NULL,
    buy_price  BIGINT NOT NULL,
    price      BIGINT NOT NULL,
    qty        INTEGER NOT NULL CHECK (qty > 0)
);

CREATE INDEX IF NOT EXISTS idx_trx_items_trx ON transaction_items (trx_id);

CREATE TABLE IF NOT EXISTS refunds (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id   UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    trx_id     TEXT NOT NULL REFERENCES transactions(id),
    items      JSONB NOT NULL, -- [{"productId":"…","qty":n}]
    reason     TEXT NOT NULL,
    by_name    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_refunds_trx ON refunds (trx_id);
