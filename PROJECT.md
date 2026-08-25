# OpenPOS Backend — PROJECT.md

Backend OpenPOS: Point of Sale gratis untuk UMKM Indonesia.
Arsitektur tradisional — **frontend → backend (REST) → PostgreSQL (Supabase)**. Tidak memakai auto-API Supabase.

> Dokumen ini adalah **kontrak API**. Setiap endpoint/field baru wajib dicatat di sini.
> Panduan untuk tim frontend ada di `EXPECTED.md`.

---

## Status

- **Fase:** MVP Backend SELESAI ✅ — seluruh modul P0 terimplementasi & teruji
- **Arsitektur:** frontend → backend REST (Go/chi) → PostgreSQL (Supabase)
- **Frontend:** seluruh halaman inti (auth, users, produk, stok, POS, transaksi) memakai API; Dashboard/Laporan/Pengaturan menyusul penyempurnaan agregat bila dibutuhkan

## Teknologi

| Bagian | Pilihan |
|---|---|
| Bahasa | Go 1.27 |
| Router | [chi v5](https://github.com/go-chi/chi) (+ cors) |
| DB driver | [pgx/v5](https://github.com/jackc/pgx) (pool, **`QueryExecModeSimpleProtocol` dipaksa di kode** agar kompatibel transaction pooler Supabase/PgBouncer) |
| Auth | JWT HS256 access token + refresh token (rotasi, hash SHA-256 di DB) |
| Hash password/passcode | bcrypt |

## Struktur

```
backend/
├── cmd/api/main.go          entrypoint: router, CORS, graceful shutdown
├── internal/
│   ├── config/              env loader (.env didukung via godotenv)
│   ├── db/                  koneksi pool + runner migrasi embedded
│   ├── model/               struct User/Store/TokenPair
│   ├── repo/                query SQL (users, refresh_tokens)
│   ├── service/             logika auth (bcrypt, JWT)
│   ├── handler/             HTTP handler (auth, health)
│   └── middleware/          Bearer auth, RequireRole (siap untuk RBAC modul berikutnya)
├── migrations/              001_init.sql (stores, users, refresh_tokens) + embed.go
├── .env.example
└── PROJECT.md / EXPECTED.md
```

## Menjalankan

```bash
cp .env.example .env      # isi DATABASE_URL (Supabase pooler) & JWT_SECRET
go run ./cmd/api          # default http://localhost:8080
```

Migrasi skema dijalankan otomatis saat startup (tabel `schema_migrations` melacak versi).

### Utility `cmd/hashpw`
Membuat bcrypt hash untuk insert akun manual via Supabase Dashboard:
```bash
go run ./cmd/hashpw kata-sandi-anda   # output: $2a$10$… → tempel ke kolom password_hash
```

### Variabel lingkungan

| Variabel | Wajib | Default | Keterangan |
|---|---|---|---|
| `PORT` | tidak | `8080` | port HTTP |
| `DATABASE_URL` | **ya** | — | connection string Postgres (Supabase: pakai pooler port `6543`) |
| `JWT_SECRET` | **ya** | — | secret HS256 (`openssl rand -hex 32`) |
| `ACCESS_TTL_MINUTES` | tidak | `15` | umur access token |
| `REFRESH_TTL_DAYS` | tidak | `7` | umur refresh token |
| `CORS_ORIGINS` | tidak | `http://localhost:5173` | origin frontend, dipisah koma. URL produksi (Vercel) ditambahkan di sini nanti |

---

# Kontrak API

Base URL pengembangan: `http://localhost:8080/api/v1`
Semua body: JSON. Semua error: `{ "error": "pesan" }` dengan HTTP code sesuai.

## Autentikasi

Access token dikirim via header: `Authorization: Bearer <access_token>`.
Refresh token disimpan klien; dipakai hanya ke `/auth/refresh`.

### `GET /health`
Cek layanan & database.
```json
{ "status": "ok", "database": "up", "service": "openpos-backend" }
```

### `POST /auth/register` — publik
Membuat **Store + akun Admin** sekaligus lalu langsung login.

Request:
```json
{ "name": "Bu Sari", "email": "sari@tokosaya.com", "password": "minimal8char", "storeName": "Toko Sembako Sari" }
```

Validasi server: email format, password ≥ 8 karakter, semua field wajib.

Response `201 Created`:
```json
{
  "access_token": "eyJ…",
  "refresh_token": "hex64…",
  "user": {
    "id": "uuid",
    "email": "sari@tokosaya.com",
    "name": "Bu Sari",
    "role": "admin",
    "store_id": "uuid",
    "store_name": "Toko Sembako Sari"
  }
}
```

Error: `400` validasi gagal · `409` `"Email sudah terdaftar. Silakan masuk."`

### `POST /auth/login` — publik
```json
{ "email": "sari@tokosaya.com", "password": "…", "passcode": "12345" }
```
`passcode` opsional — hanya dikirim pada percobaan kedua.

Response `200`: sama bentuknya dengan register.

Error spesial:
- Akun punya passcode tapi request belum mengirimkannya → `401 { "error": "passcode_required" }`
  → klien harus menampilkan input passcode lalu login ulang dengan `passcode`.
- Kredensial salah → `401` `"Email atau kata sandi tidak cocok. Coba lagi."`
- Passcode salah → `401` `"Passcode salah. Coba lagi."`
- Akun dinonaktifkan → `403` `"Akun dinonaktifkan. Hubungi admin toko."`

### `POST /auth/refresh` — publik
Rotasi: refresh token lama dicabut, pasangan baru diterbitkan.
```json
{ "refresh_token": "hex64…" }
```
Response `200`: sama dengan login. Error `401` bila token revoked/expired/user nonaktif.

### `POST /auth/logout` — publik
Mencabut refresh token. Body opsional:
```json
{ "refresh_token": "hex64…" }
```
Response `200 { "message": "keluar berhasil" }`. Klien tetap wajib menghapus token lokalnya.

### `GET /auth/me` — 🔒 Bearer
Profil user sesi aktif (role & status aktif dibaca fresh dari DB).
Response `200`:
```json
{ "user": { "id": "…", "email": "…", "name": "…", "role": "admin", "store_id": "…", "store_name": "Toko Sembako Sari" } }
```
Error `401` bila token tidak valid.

---

## User Management (admin) 🔒

Semua endpoint di bawah wajib Bearer token dengan `role: "admin"` — kasir mendapat `403`.
Seluruh data otomatis ter-scope ke toko admin (dari JWT claim `sid`).

Bentuk objek user:
```json
{ "id": "uuid", "email": "…", "name": "…", "role": "admin|cashier",
  "active": true, "store_id": "uuid", "store_name": "…", "created_at": "RFC3339" }
```

### `GET /users` 🔒 admin
Daftar seluruh akun dalam toko (tertua dulu). Response: `{ "users": [ … ] }`

### `POST /users` 🔒 admin
Membuat akun **kasir** baru. Request:
```json
{ "name": "Andi", "email": "andi@tokosaya.com", "password": "minimal8char" }
```
Response `201 { "user": … }`. Error: `400` validasi · `409` email sudah terdaftar.

### `PATCH /users/{id}/active` 🔒 admin
Mengaktifkan/menonaktifkan akun kasir. Request: `{ "active": false }`
- Kasir nonaktif langsung ditolak saat login (`403`) — FR-USR-004
- Akun admin tidak dapat diubah lewat endpoint ini → `400`
- ID milik toko lain → `404` (tidak dibocorkan keberadaannya)

Response `200 { "message": "Akun dinonaktifkan." }`

---

## Katalog: Kategori & Produk

Uang disimpan `bigint` rupiah tanpa pecahan. SKU **unik per toko, tidak peka huruf besar/kecil**.
Semua endpoint 🔒 Bearer. Baca terbuka untuk admin & kasir (POS), tulis hanya admin.

Objek kategori:
```json
{ "id": "uuid", "name": "Sembako", "active": true, "created_at": "RFC3339" }
```

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| GET | /categories | semua | daftar kategori toko (termasuk nonaktif) |
| POST | /categories | admin | `{ "name": "…" }` → 201; duplikat nama → 409 |
| DELETE | /categories/{id} | admin | masih dipakai produk → soft-delete (`{"soft_deleted":true}`); tidak dipakai → hard delete |

Objek produk:
```json
{ "id": "uuid", "category_id": "uuid|null", "category_name": "…|null",
  "name": "Beras 5kg", "sku": "BR-001", "barcode": "8991…",
  "buy_price": 62000, "sell_price": 68000, "stock": 24,
  "unit": "pcs", "active": true, "created_at": "RFC3339" }
```

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| GET | /products?q=&categoryId=&active=&page=&limit= | semua | pagination server-side (maks limit 200); cari di nama/SKU/barcode |
| GET | /products/{id} | semua | detail |
| POST | /products | admin | buat; `stock` hanya saat create; SKU duplikat → 409 |
| PUT | /products/{id} | admin | ubah atribut — **stok tidak diubah di sini** |
| PATCH | /products/{id}/active | admin | `{ "active": false }` |

Error khas modul ini: `409 SKU sudah digunakan di toko ini.` · `400 Kategori tidak ditemukan di toko Anda.` · ID milik toko lain → `404`.

---

## Transaksi & Refund

Checkout dihitung **penuh di server** (harga dari DB, bukan dari klien), stok & movement & nomor
transaksi ditulis dalam satu transaksi DB dengan advisory-lock per toko (aman multi-kasir, EC-001).
ID format `TRX-0001` (seq per toko). Pajak memakai `stores.tax_enabled`/`tax_pct`.

Objek transaksi:
```json
{ "id":"TRX-0001","seq":1,"cashier_name":"Kasir Test",
  "items":[{"product_id":"uuid","name":"snapshot nama","buy_price":0,"price":2000,"qty":3}],
  "subtotal":6000,"discount":500,"tax":0,"total":5500,
  "method":"Cash","paid":10000,"change":4500,
  "status":"completed|pending|cancelled|refunded",
  "customer":"","time":"RFC3339" }
```

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| POST | /transactions | semua | `{ items:[{productId,qty}], discount?, method, paid?, customer? }` → 201 objek trx. Non-cash: `paid` dipaksa = total. Error: keranjang kosong/bayar kurang/diskon melebihi → 400 · stok kurang → 409 |
| GET | /transactions?q=&method=&date=YYYY-MM-DD&page=&limit= | semua | kasir otomatis hanya miliknya (FR-TRX-002); item disertakan |
| GET | /transactions/{id} | semua | detail; milik orang lain (kasir) → 404 |
| POST | /transactions/{id}/refund | admin | `{ items:[{productId,qty}], reason }` — parsial/penuh (FR-REF-003); stok kembali + movement 'refund'; penuh → status `refunded`; dobel/kelebihan ditolak (EC-004) |

---

## Stok & Riwayat Pergerakan

Objek movement:
```json
{ "id": "uuid", "product_id": "uuid", "product_name": "Beras 5kg",
  "type": "sale|refund|adjust|initial", "qty": -2,
  "reason": "barang rusak", "actor": "Bu Sari", "created_at": "RFC3339" }
```
`qty` negatif = stok keluar, positif = masuk.

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| GET | /movements?type=&productId=&page=&limit= | admin | riwayat toko, terbaru dulu |
| POST | /stock/adjustments | admin | `{ productId, direction:"plus"\|"minus", qty, reason }` |

Aturan penyesuaian (FR-INV-002/003/006/007): alasan **wajib**; hasil akhir dilarang negatif → `400 {"error":"stok tidak boleh negatif"}`; stok & movement ditulis dalam **satu transaksi DB**; produk baru dengan stok awal > 0 otomatis mendapat movement `initial`. Kasir memanggil endpoint ini → `403`; ID milik toko lain → `404`.

---

## Settings Toko

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| GET | /settings | semua role | profil toko + struk + pajak (struk kasir butuh ini) |
| PUT | /settings | admin | `{ storeName, address, phone, taxEnabled, taxPct, receiptHeader, receiptFooter, paper:"58mm"\|80mm", timezone }` |
| PUT | /users/{id}/passcode | admin | `{ passcode:"12345" }` set · `{ passcode:"" }` hapus |

Perubahan pajak langsung memengaruhi perhitungan checkout berikutnya.

## Dashboard & Laporan

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| GET | /dashboard | semua role | role-aware: admin → KPI hari ini + grafik 7 hari + metode bayar + top produk + recent (zona waktu toko); kasir → ringkasan shift sendiri saja (FR-DASH-003) |
| GET | /reports?period=today\|yesterday\|week\|month\|all | admin | komposit: summary (omzet/trx/item/profit), by_method, by_status, products, transactions (total/hpp/profit), stock (nilai stok). HPP dari snapshot item (EC-008) |

## Skema database

```sql
stores         (id uuid PK, name, created_at)
users          (id uuid PK, store_id FK→stores ON DELETE CASCADE, email UNIQUE,
                name, password_hash, passcode_hash NULLABLE,
                role 'admin'|'cashier', active bool, created_at)
refresh_tokens (id uuid PK, user_id FK→users ON DELETE CASCADE,
                token_hash UNIQUE (sha256), expires_at, revoked, created_at)
categories     (id uuid PK, store_id FK→stores ON DELETE CASCADE,
                name, active bool, created_at, UNIQUE(store_id,name))
products       (id uuid PK, store_id FK→stores ON DELETE CASCADE,
                category_id FK→categories ON DELETE SET NULL,
                name, sku, barcode, buy_price bigint, sell_price bigint,
                stock int CHECK(>=0), unit 'pcs', active bool, created_at,
                UNIQUE(store_id, lower(sku)))
stock_movements(id uuid PK, store_id FK→stores ON DELETE CASCADE,
                product_id FK→products ON DELETE CASCADE,
                type 'sale|refund|adjust|initial', qty int (±),
                reason, actor, created_at)
transactions   (id 'TRX-0001' PK, seq int, store_id FK, cashier_id FK→users,
                cashier_name, subtotal/discount/tax/total bigint,
                method enum5, paid, change, status enum4, customer, created_at,
                UNIQUE(store_id, seq))
transaction_items(trx_id FK→transactions CASCADE, product_id FK,
                name/buy_price/price SNAPSHOT, qty>0)
refunds        (id uuid PK, store_id FK, trx_id FK, items JSONB [{productId,qty}],
                reason, by_name, created_at)
-- kolom settings di stores (migrasi 005):
--   address, phone, tax_enabled, tax_pct, receipt_header,
--   receipt_footer, paper, timezone
```

Semua FK memakai `ON DELETE CASCADE` — menghapus baris langsung dari Supabase Dashboard
(mis. hapus store) otomatis membersihkan users dan refresh_tokens terkait.

Catatan desain:
- Uang (modul berikutnya): `bigint` rupiah tanpa pecahan (NFR-003).
- Semua query modul bisnis akan di-scope `store_id` dari JWT claim.
- Ganti akun antar-perangkat = logout + login penuh (tidak ada switch tanpa re-auth).

## Roadmap modul — SELESAI ✅

1. ~~Auth~~ ✅
2. ~~Users — admin kelola kasir~~ ✅
3. ~~Produk + Kategori~~ ✅
4. ~~Stok + movement~~ ✅
5. ~~Transaksi atomik + Refund~~ ✅
6. ~~Laporan + Dashboard + Settings~~ ✅

### Ide lanjutan (pasca-MVP, sesuai PRD §14)
Forgot-password/email verification · audit log UI · import CSV/XLSX server-side · export PDF struk · realtime sync · multi-store · offline POS · payment gateway.
