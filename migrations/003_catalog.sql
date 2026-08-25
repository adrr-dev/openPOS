-- Katalog: kategori & produk (uang bigint rupiah, stok >= 0)
CREATE TABLE IF NOT EXISTS categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id   UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, name)
);

CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id    UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    sku         TEXT NOT NULL,
    barcode     TEXT NOT NULL DEFAULT '',
    buy_price   BIGINT NOT NULL DEFAULT 0 CHECK (buy_price >= 0),
    sell_price  BIGINT NOT NULL DEFAULT 0 CHECK (sell_price >= 0),
    stock       INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
    unit        TEXT NOT NULL DEFAULT 'pcs',
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- SKU unik dalam lingkup satu toko, tidak peka huruf besar/kecil (FR-PROD-003)
CREATE UNIQUE INDEX IF NOT EXISTS uq_products_store_sku ON products (store_id, lower(sku));
CREATE INDEX IF NOT EXISTS idx_products_store ON products (store_id);
CREATE INDEX IF NOT EXISTS idx_categories_store ON categories (store_id);
