-- Riwayat pergerakan stok (FR-INV-003 s.d. FR-INV-005)
CREATE TABLE IF NOT EXISTS stock_movements (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id   UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('sale', 'refund', 'adjust', 'initial')),
    qty        INTEGER NOT NULL, -- negatif = keluar, positif = masuk
    reason     TEXT NOT NULL DEFAULT '',
    actor      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_movements_store_time ON stock_movements (store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_movements_product ON stock_movements (product_id);
