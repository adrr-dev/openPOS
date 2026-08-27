# openPOS — Backend API

Backend REST untuk OpenPOS. Stack: **Go (chi) + PostgreSQL**.

Base URL: `http://localhost:8080/api/v1`

Semua body: JSON. Semua error: `{ "error": "pesan" }`.

---

## Autentikasi

Access token dikirim via header: `Authorization: Bearer <access_token>`.

### `GET /health`

Cek layanan & database.

```json
{ "status": "ok", "database": "up", "service": "openpos-backend" }
```

### `POST /auth/register`

Buat Store + akun Admin sekaligus, langsung login.

**Request:**
```json
{ "name": "Bu Sari", "email": "sari@tokosaya.com", "password": "minimal8char", "storeName": "Toko Sembako Sari" }
```

**Response `201`:**
```json
{
  "access_token": "eyJ…",
  "refresh_token": "hex64…",
  "user": {
    "id": "uuid",
    "email": "sari@tokosaya.com",
    "name": "Bu Sari",
    "role": "admin",
    "active": true,
    "store_id": "uuid",
    "store_name": "Toko Sembako Sari"
  }
}
```

**Error:** `400` validasi gagal · `409` email sudah terdaftar

### `POST /auth/login`

**Request:**
```json
{ "email": "sari@tokosaya.com", "password": "…", "passcode": "12345" }
```

`passcode` opsional — hanya dikirim pada percobaan kedua jika akun punya passcode.

**Response `200`:** sama dengan register.

**Error spesial:**
- `401 { "error": "passcode_required" }` — akun punya passcode tapi belum dikirim
- `401` kredensial salah
- `401` passcode salah
- `403` akun dinonaktifkan

### `POST /auth/refresh`

Rotasi refresh token.

**Request:**
```json
{ "refresh_token": "hex64…" }
```

**Response `200`:** sama dengan login. Error `401` bila token revoked/expired/user nonaktif.

### `POST /auth/logout`

Mencabut refresh token. Body opsional.

**Request:**
```json
{ "refresh_token": "hex64…" }
```

**Response `200`:** `{ "message": "keluar berhasil" }`

### `GET /auth/me`

Profil user sesi aktif.

**Response `200`:**
```json
{
  "user": {
    "id": "uuid",
    "email": "sari@tokosaya.com",
    "name": "Bu Sari",
    "role": "admin",
    "active": true,
    "store_id": "uuid",
    "store_name": "Toko Sembako Sari"
  }
}
```

### `POST /auth/switch`

Beralih sesi ke akun lain dalam toko yang sama. Jika target punya passcode, wajib validasi. Jika tidak ada passcode, switch langsung berhasil.

**Request:**
```json
{ "target_user_id": "uuid-kasir", "passcode": "12345" }
```

`passcode` opsional — wajib hanya jika target punya passcode.

**Response `200`:**
```json
{
  "access_token": "eyJ…",
  "refresh_token": "hex64…",
  "user": {
    "id": "uuid",
    "email": "andi@tokosaya.com",
    "name": "Andi",
    "role": "cashier",
    "active": true,
    "store_id": "uuid",
    "store_name": "Toko Sembako Sari"
  }
}
```

**Error:**
- `400` switch ke akun sendiri
- `401` passcode_required
- `401` passcode salah
- `403` akun dinonaktifkan
- `404` akun tidak ditemukan / beda toko

---

## User Management 🔒 admin

Semua endpoint wajib Bearer token dengan `role: "admin"`.

Bentuk objek user:
```json
{
  "id": "uuid",
  "email": "andi@tokosaya.com",
  "name": "Andi",
  "role": "admin|cashier",
  "active": true,
  "store_id": "uuid",
  "store_name": "Toko Sembako Sari",
  "created_at": "2026-01-15T10:30:00Z"
}
```

### `GET /users`

Daftar seluruh akun dalam toko (tertua dulu).

**Response `200`:** `{ "users": [ … ] }`

### `POST /users`

Buat akun kasir baru.

**Request:**
```json
{ "name": "Andi", "email": "andi@tokosaya.com", "password": "minimal8char" }
```

**Response `201`:** `{ "user": { … } }`

**Error:** `400` validasi · `409` email sudah terdaftar

### `PATCH /users/{id}/active`

Aktifkan/nonaktifkan akun kasir.

**Request:**
```json
{ "active": false }
```

**Response `200`:** `{ "message": "Akun dinonaktifkan." }`

**Error:** `400` akun admin tidak bisa diubah · `404` tidak ditemukan

### `PUT /users/{id}/passcode`

Set atau hapus passcode akun.

**Request set:** `{ "passcode": "12345" }`
**Request hapus:** `{ "passcode": "" }`

**Response `200`:** `{ "message": "Passcode disimpan." }` atau `{ "message": "Passcode dihapus." }`

**Error:** `400` passcode harus 5 angka · `404` tidak ditemukan

---

## Katalog 🔒

Semua endpoint Bearer. Baca terbuka untuk admin & kasir, tulis hanya admin.

### Kategori

**Objek:**
```json
{ "id": "uuid", "name": "Sembako", "active": true, "created_at": "2026-01-15T10:30:00Z" }
```

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| GET | /categories | semua | daftar kategori toko |
| POST | /categories | admin | `{ "name": "…" }` → 201; duplikat → 409 |
| DELETE | /categories/{id} | admin | masih dipakai produk → soft-delete; tidak dipakai → hard delete |

### Produk

**Objek:**
```json
{
  "id": "uuid",
  "category_id": "uuid|null",
  "category_name": "Sembako|null",
  "name": "Beras 5kg",
  "sku": "BR-001",
  "barcode": "8991…",
  "buy_price": 62000,
  "sell_price": 68000,
  "stock": 24,
  "unit": "pcs",
  "active": true,
  "created_at": "2026-01-15T10:30:00Z"
}
```

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| GET | /products?q=&categoryId=&active=&page=&limit= | semua | pagination (maks 200); cari di nama/SKU/barcode |
| GET | /products/{id} | semua | detail |
| POST | /products | admin | `{ name, sku, barcode?, categoryId?, buyPrice?, sellPrice, stock?, unit? }` → 201 |
| PUT | /products/{id} | admin | ubah atribut — stok tidak diubah di sini |
| PATCH | /products/{id}/active | admin | `{ "active": false }` |

**Error:** `409` SKU sudah digunakan · `400` kategori tidak ditemukan

---

## Stok & Pergerakan 🔒 admin

**Objek movement:**
```json
{
  "id": "uuid",
  "product_id": "uuid",
  "product_name": "Beras 5kg",
  "type": "sale|refund|adjust|initial",
  "qty": -2,
  "reason": "barang rusak",
  "actor": "Bu Sari",
  "created_at": "2026-01-15T10:30:00Z"
}
```

`qty` negatif = stok keluar, positif = masuk.

| Method | Path | Keterangan |
|---|---|---|
| GET | /movements?type=&productId=&page=&limit= | riwayat toko, terbaru dulu |
| POST | /stock/adjustments | `{ productId, direction: "plus"\|"minus", qty, reason }` |

**Aturan:** alasan wajib; stok akhir dilarang negatif → `400`; stok & movement ditulis dalam satu transaksi DB.

---

## Transaksi 🔒

**Objek transaksi:**
```json
{
  "id": "TRX-0001",
  "seq": 1,
  "cashier_name": "Andi",
  "items": [
    { "product_id": "uuid", "name": "Beras 5kg", "buy_price": 62000, "price": 68000, "qty": 2 }
  ],
  "subtotal": 136000,
  "discount": 5000,
  "tax": 0,
  "total": 131000,
  "method": "Cash",
  "paid": 150000,
  "change": 19000,
  "status": "completed|pending|cancelled|refunded",
  "customer": "",
  "time": "2026-01-15T10:30:00Z"
}
```

| Method | Path | Peran | Keterangan |
|---|---|---|---|
| POST | /transactions | semua | `{ items:[{productId,qty}], discount?, method, paid?, customer? }` → 201 |
| GET | /transactions?q=&method=&date=YYYY-MM-DD&page=&limit= | semua | kasir hanya lihat miliknya; item disertakan |
| GET | /transactions/{id} | semua | detail |
| POST | /transactions/{id}/refund | admin | `{ items:[{productId,qty}], reason }` — parsial/penuh |

**Error checkout:** `400` keranjang kosong/bayar kurang · `409` stok kurang

---

## Pengaturan Toko 🔒

### `GET /settings`

Profil toko + struk + pajak. Semua role.

**Response `200`:**
```json
{
  "storeName": "Toko Sembako Sari",
  "address": "Jl. Merdeka No. 1",
  "phone": "081234567890",
  "taxEnabled": true,
  "taxPct": 10,
  "receiptHeader": "Terima kasih sudah berbelanja",
  "receiptFooter": "Barang yang sudah dibeli tidak dapat ditukar",
  "paper": "58mm",
  "timezone": "Asia/Makassar"
}
```

### `PUT /settings`

Admin only.

**Request:**
```json
{
  "storeName": "Toko Sembako Sari",
  "address": "Jl. Merdeka No. 1",
  "phone": "081234567890",
  "taxEnabled": true,
  "taxPct": 10,
  "receiptHeader": "Terima kasih sudah berbelanja",
  "receiptFooter": "Barang yang sudah dibeli tidak dapat ditukar",
  "paper": "58mm",
  "timezone": "Asia/Makassar"
}
```

**Response `200`:** objek settings yang sudah diupdate.

---

## Dashboard 🔒

### `GET /dashboard`

Role-aware: admin → KPI + grafik 7 hari + metode bayar + top produk + recent; kasir → ringkasan shift sendiri.

**Response admin:**
```json
{
  "role": "admin",
  "today": { "omzet": 1500000, "trx_count": 12, "items_sold": 34, "low_stock": 3 },
  "sales7": [ { "date": "2026-01-15", "omzet": 1500000 } ],
  "methods": [ { "method": "Cash", "total": 800000 } ],
  "top_products": [ { "product_id": "uuid", "name": "Beras 5kg", "qty": 10, "revenue": 680000 } ],
  "recent": [ { "id": "TRX-0012", "cashier_name": "Andi", "total": 131000, "status": "completed", "time": "2026-01-15T10:30:00Z" } ]
}
```

**Response kasir:**
```json
{
  "role": "cashier",
  "today": { "omzet": 500000, "trx_count": 5, "items_sold": 12 },
  "recent": [ { "id": "TRX-0010", "cashier_name": "Andi", "total": 50000, "status": "completed", "time": "…" } ]
}
```

---

## Laporan 🔒 admin

### `GET /reports?period=today|yesterday|week|month|all`

**Response `200`:**
```json
{
  "period": "today",
  "summary": { "omzet": 1500000, "trx_count": 12, "items_sold": 34, "gross_profit": 300000 },
  "by_method": [ { "method": "Cash", "total": 800000 } ],
  "by_status": [ { "status": "completed", "count": 10 } ],
  "products": [ { "product_id": "uuid", "name": "Beras 5kg", "sku": "BR-001", "qty": 10, "revenue": 680000, "profit": 60000 } ],
  "transactions": [ { "date": "2026-01-15", "id": "TRX-0001", "cashier": "Andi", "method": "Cash", "total": 131000, "hpp": 124000, "profit": 7000, "status": "completed" } ],
  "stock": [ { "name": "Beras 5kg", "sku": "BR-001", "stock": 24, "buy_price": 62000, "sell_price": 68000, "stock_value": 1488000 } ]
}
```

---

## Environment Variables

| Variabel | Wajib | Default | Keterangan |
|---|---|---|---|
| `PORT` | tidak | `8080` | port HTTP |
| `DATABASE_URL` | **ya** | — | connection string PostgreSQL |
| `JWT_SECRET` | **ya** | — | secret HS256 |
| `ACCESS_TTL_MINUTES` | tidak | `15` | umur access token |
| `REFRESH_TTL_DAYS` | tidak | `7` | umur refresh token |
| `CORS_ORIGINS` | tidak | `http://localhost:5173` | origin frontend, dipisah koma |

---

## Menjalankan

```bash
cp .env.example .env    # isi nilai
make run                # http://localhost:8080/api/v1
```
