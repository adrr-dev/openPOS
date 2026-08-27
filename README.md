# openPOS — Backend API

Backend REST API untuk openPOS. Dibangun menggunakan **Go (chi) + PostgreSQL** dan siap di-deploy secara serverless di Vercel atau menggunakan server jangka panjang VPS.

- **Base URL:** `https://openpos-api.vercel.app/api/v1`
- **Format Payload:** `application/json` (Semua body request dan response berbentuk JSON)
- **Format Error:** Semua kegagalan API akan mengembalikan payload error standar:
  ```json
  { "error": "Pesan deskripsi kesalahan detail di sini" }
  ```

---

## 💡 Informasi Tambahan untuk Frontend (Developer Tips)

Sebelum mengintegrasikan UI, harap perhatikan pedoman berikut agar integrasi berjalan mulus:

### 1. Siklus Hidup Autentikasi (Token Lifecycle)
* **AccessToken (JWT):** Memiliki masa aktif singkat (~15 menit). Kirimkan via header HTTP:  
  `Authorization: Bearer <access_token>`
* **RefreshToken:** Memiliki masa aktif panjang (~7 hari). Simpan secara aman di client (`localStorage` / secure storage).
* **Silent Token Rotation:** Sangat disarankan untuk membuat *HTTP interceptor* (seperti pada Axios/Fetch). Jika request mengembalikan `401` dengan pesan kedaluwarsa, panggil `POST /auth/refresh` secara otomatis di belakang layar untuk mendapatkan pasangan token baru, lalu ulangi request yang gagal tadi tanpa mengganggu pengalaman pengguna.

### 2. Alur Pembagian Sesi Akun (Multi-Cashier & PIN 5 Angka)
Aplikasi ini mendukung perpindahan kasir cepat (common di mesin POS kasir fisik) menggunakan PIN/Passcode 5 digit.
* **Alur Login / Switch Akun:**
  1. Jalankan request login/switch terlebih dahulu tanpa mengirimkan parameter `passcode`.
  2. Jika akun dilindungi oleh passcode, server akan merespons dengan status `401 Unauthorized` dan body `{"error": "passcode_required"}`.
  3. Ketika frontend menerima error tersebut, tampilkan modal/popup keypad PIN 5-digit ke layar.
  4. Pengguna memasukkan PIN, kemudian frontend mengulangi request dengan menyertakan atribut `passcode: "12345"`.

### 3. Presisi Perhitungan POS & Pajak (Tax)
* **Snapshot Harga:** Backend selalu merekam snapshot `buy_price` dan `sell_price` saat transaksi terjadi. Nilai profit/laba rugi masa lalu tidak akan berubah meskipun harga produk diedit di masa depan.
* **Perhitungan Pajak:** Pajak dihitung otomatis oleh server jika `taxEnabled` bernilai `true` pada `/settings`.  
  Rumus perhitungan pajak backend:  
  `tax = round_half_up((subtotal - discount) * taxPct / 100)`
* **Nilai Uang (Currency):** Semua nilai uang seperti harga beli, harga jual, subtotal, diskon, pajak, total belanja, jumlah bayar, kembalian, dan laba kotor disimpan dalam tipe data **Integer** 64-bit (tidak menggunakan desimal/sen).

### 4. Pembatasan Hak Akses (Role-Aware)
* Pihak Kasir (`cashier`) secara sistem otomatis dibatasi oleh backend. Endpoint `GET /transactions` dan `GET /dashboard` secara implisit hanya akan memunculkan data milik kasir yang sedang aktif tersebut.
* Halaman pengaturan toko, pembuatan produk/kategori, manipulasi stok, refund transaksi, manajemen akun kasir, dan penarikan laporan periodik wajib di-lock/sembunyikan di sisi frontend bila pengguna yang masuk memiliki `role: "cashier"`.

### 5. Cetak Struk POS (Receipt Printing)
* Tarik konfigurasi cetak struk dari endpoint `/settings`. Perhatikan atribut `paper` (`58mm` atau `80mm`) untuk menyesuaikan layout CSS printing atau byte stream ESC/POS printer termal.

---

## 🛠️ Daftar Lengkap API Endpoints

### 🩺 Layanan & Health Check

#### `GET /health`
Mengecek status kesehatan aplikasi, koneksi database, dan konektivitas API.
* **Autentikasi:** Publik (Tanpa token)
* **Response Sukses (`200 OK`):**
  ```json
  {
    "status": "ok",
    "database": "up",
    "service": "openpos-backend"
  }
  ```

---

### 🔑 Autentikasi (Authentication)

#### `POST /auth/register`
Mendaftarkan Toko (Store) baru sekaligus membuat akun Admin pertama untuk toko tersebut, lalu otomatis login (mengembalikan token).
* **Autentikasi:** Publik (Tanpa token)
* **Request Expected:**
  ```json
  {
    "name": "Bu Sari",
    "email": "sari@tokosaya.com",
    "password": "passwordminimal8karakter",
    "storeName": "Toko Sembako Sari"
  }
  ```
* **Response Sukses (`201 Created`):**
  ```json
  {
    "access_token": "eyJ...",
    "refresh_token": "88cb...",
    "user": {
      "id": "e4414f52-8703-45db-99bd-fa02a0a2df3c",
      "email": "sari@tokosaya.com",
      "name": "Bu Sari",
      "role": "admin",
      "active": true,
      "store_id": "df2a0752-6cfa-42f5-b6d4-83b632617a2d",
      "store_name": "Toko Sembako Sari"
    }
  }
  ```
* **Expected Errors:**
  * `400 Bad Request` — Validasi gagal (misal: email kosong/tidak valid, password kurang dari 8 karakter).
  * `409 Conflict` — `{"error": "Email sudah terdaftar. Silakan masuk."}`

#### `POST /auth/login`
Autentikasi masuk pengguna menggunakan email dan password.
* **Autentikasi:** Publik (Tanpa token)
* **Request Expected:**
  ```json
  {
    "email": "sari@tokosaya.com",
    "password": "passwordminimal8karakter",
    "passcode": "12345" 
  }
  ```
  *(Catatan: Atribut `passcode` opsional, dikirim hanya jika server merespons dengan tantangan PIN)*
* **Response Sukses (`200 OK`):** format response sama dengan `/auth/register`.
* **Expected Errors:**
  * `401 Unauthorized` — `{"error": "passcode_required"}` (Akun dilindungi passcode/PIN, minta input PIN 5 digit dari user).
  * `401 Unauthorized` — `{"error": "Email atau kata sandi tidak cocok. Coba lagi."}`
  * `401 Unauthorized` — `{"error": "Passcode salah. Coba lagi."}`
  * `403 Forbidden` — `{"error": "Akun dinonaktifkan. Hubungi admin toko."}`

#### `POST /auth/refresh`
Melakukan rotasi/pembaruan Access Token yang telah kedaluwarsa menggunakan Refresh Token yang valid.
* **Autentikasi:** Publik (Tanpa token)
* **Request Expected:**
  ```json
  {
    "refresh_token": "88cb..."
  }
  ```
* **Response Sukses (`200 OK`):** Mengembalikan pasangan token baru beserta profil pengguna (format sama dengan `/auth/register`).
* **Expected Errors:**
  * `401 Unauthorized` — `{"error": "sesi tidak valid, silakan masuk kembali"}`

#### `POST /auth/logout`
Mencabut status aktif dari Refresh Token agar tidak bisa digunakan lagi di masa mendatang.
* **Autentikasi:** Publik (Tanpa token, body opsional)
* **Request Expected:**
  ```json
  {
    "refresh_token": "88cb..."
  }
  ```
* **Response Sukses (`200 OK`):**
  ```json
  {
    "message": "keluar berhasil"
  }
  ```

#### `GET /auth/me`
Mengambil profil data diri pengguna yang saat ini sedang aktif dalam sesi token.
* **Autentikasi:** Bearer Token (Semua Role)
* **Response Sukses (`200 OK`):**
  ```json
  {
    "user": {
      "id": "e4414f52-8703-45db-99bd-fa02a0a2df3c",
      "email": "sari@tokosaya.com",
      "name": "Bu Sari",
      "role": "admin",
      "active": true,
      "store_id": "df2a0752-6cfa-42f5-b6d4-83b632617a2d",
      "store_name": "Toko Sembako Sari"
    }
  }
  ```
* **Expected Errors:**
  * `401 Unauthorized` — `{"error": "sesi tidak valid, silakan masuk kembali"}`

#### `POST /auth/switch`
Beralih sesi aktif ke akun kasir/admin lain dalam toko yang sama secara instan (fast login switch).
* **Autentikasi:** Bearer Token (Semua Role)
* **Request Expected:**
  ```json
  {
    "target_user_id": "b3e211da-e0c1-4b13-aa8d-8eb99a4e69bb",
    "passcode": "54321"
  }
  ```
  *(Catatan: Atribut `passcode` opsional, hanya wajib jika akun target dilindungi oleh passcode/PIN)*
* **Response Sukses (`200 OK`):** Mengembalikan token pair baru untuk akun target (format sama dengan `/auth/register`).
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "Tidak dapat beralih ke akun sendiri."}`
  * `401 Unauthorized` — `{"error": "passcode_required"}` (Akun target butuh PIN 5 digit).
  * `401 Unauthorized` — `{"error": "Passcode salah. Coba lagi."}`
  * `403 Forbidden` — `{"error": "Akun dinonaktifkan."}`
  * `404 Not Found` — `{"error": "Akun tidak ditemukan."}`

---

### 👥 Manajemen Akun (User Management) 🔒 Admin

#### `GET /users`
Mendapatkan daftar seluruh akun staff/kasir yang terdaftar di dalam toko saat ini.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Response Sukses (`200 OK`):**
  ```json
  {
    "users": [
      {
        "id": "e4414f52-8703-45db-99bd-fa02a0a2df3c",
        "email": "sari@tokosaya.com",
        "name": "Bu Sari",
        "role": "admin",
        "active": true,
        "store_id": "df2a0752-6cfa-42f5-b6d4-83b632617a2d",
        "store_name": "Toko Sembako Sari",
        "created_at": "2026-08-25T08:15:00Z"
      },
      {
        "id": "b3e211da-e0c1-4b13-aa8d-8eb99a4e69bb",
        "email": "andi@tokosaya.com",
        "name": "Andi Kasir",
        "role": "cashier",
        "active": true,
        "store_id": "df2a0752-6cfa-42f5-b6d4-83b632617a2d",
        "store_name": "Toko Sembako Sari",
        "created_at": "2026-08-26T12:00:00Z"
      }
    ]
  }
  ```

#### `POST /users`
Membuat akun staff kasir baru dalam toko. Kasir tidak memerlukan email atau password, cukup nama.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "name": "Andi Kasir"
  }
  ```
* **Response Sukses (`201 Created`):**
  ```json
  {
    "user": {
      "id": "b3e211da-e0c1-4b13-aa8d-8eb99a4e69bb",
      "email": "",
      "name": "Andi Kasir",
      "role": "cashier",
      "active": true,
      "store_id": "df2a0752-6cfa-42f5-b6d4-83b632617a2d",
      "store_name": "Toko Sembako Sari",
      "created_at": "2026-08-27T10:30:00Z"
    }
  }
  ```
* **Expected Errors:**
  * `409 Conflict` — `{"error": "Email sudah terdaftar."}`

#### `PATCH /users/{id}/active`
Mengubah status aktif (aktifkan atau nonaktifkan) akun kasir.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "active": false
  }
  ```
* **Response Sukses (`200 OK`):**
  ```json
  {
    "message": "Akun dinonaktifkan."
  }
  ```
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "Hanya akun kasir yang dapat dinonaktifkan."}` (Akun admin tidak bisa dinonaktifkan lewat sini).
  * `404 Not Found` — `{"error": "Akun tidak ditemukan di toko Anda."}`

#### `PUT /users/{id}/passcode`
Mengatur atau menghapus Passcode/PIN 5-digit milik akun kasir tertentu.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "passcode": "54321"
  }
  ```
  *(Catatan: Kirim string kosong `""` untuk menghapus passcode dari akun)*
* **Response Sukses (`200 OK`):**
  ```json
  {
    "message": "Passcode disimpan."
  }
  ```
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "passcode harus 5 angka"}` (Validasi format PIN salah).
  * `404 Not Found` — `{"error": "Akun tidak ditemukan di toko Anda."}`

---

### 🗂️ Manajemen Kategori (Category)

#### `GET /categories`
Mendapatkan daftar seluruh kategori produk yang ada di toko.
* **Autentikasi:** Bearer Token (Semua Role)
* **Response Sukses (`200 OK`):**
  ```json
  {
    "categories": [
      {
        "id": "2da1fb9b-00fa-4009-847c-7f5fa9374021",
        "store_id": "df2a0752-6cfa-42f5-b6d4-83b632617a2d",
        "name": "Sembako",
        "active": true,
        "created_at": "2026-08-25T08:30:00Z"
      }
    ]
  }
  ```

#### `POST /categories` 🔒 Admin
Membuat kategori produk baru di toko.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "name": "Makanan Instan"
  }
  ```
* **Response Sukses (`201 Created`):**
  ```json
  {
    "category": {
      "id": "e0e2920f-87f5-46aa-acda-656910606b29",
      "store_id": "df2a0752-6cfa-42f5-b6d4-83b632617a2d",
      "name": "Makanan Instan",
      "active": true,
      "created_at": "2026-08-27T10:45:00Z"
    }
  }
  ```
* **Expected Errors:**
  * `409 Conflict` — `{"error": "Kategori dengan nama itu sudah ada."}`

#### `DELETE /categories/{id}` 🔒 Admin
Menghapus kategori. Jika kategori masih dikaitkan dengan produk yang ada, sistem akan melakukan *soft-delete* (menonaktifkan kategori). Jika sudah kosong, sistem melakukan *hard-delete*.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Response Sukses (`200 OK`):**
  ```json
  {
    "soft_deleted": true
  }
  ```
  *(Catatan: `soft_deleted: true` berarti kategori dinonaktifkan karena masih digunakan oleh produk. `false` berarti terhapus sepenuhnya)*

---

### 📦 Manajemen Produk (Product)

#### `GET /products`
Mendapatkan katalog daftar seluruh produk dengan opsi penyaringan, pencarian, dan pagination.
* **Autentikasi:** Bearer Token (Semua Role)
* **Query Parameters:**
  * `q` (string, opsional): Pencarian berbasis teks kecocokan parsial *case-insensitive* pada nama produk, SKU, atau barcode.
  * `categoryId` (uuid, opsional): Filter produk berdasarkan kategori tertentu.
  * `active` (string, opsional): Status produk (`"true"` / `"false"`).
  * `page` (int, opsional): Halaman keberapa (default: `1`).
  * `limit` (int, opsional): Batas item per halaman (default: `20`, maksimum: `200`).
* **Response Sukses (`200 OK`):**
  ```json
  {
    "items": [
      {
        "id": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
        "category_id": "2da1fb9b-00fa-4009-847c-7f5fa9374021",
        "category_name": "Sembako",
        "name": "Beras Premium 5kg",
        "sku": "BR-001",
        "barcode": "8991234567891",
        "buy_price": 62000,
        "sell_price": 68000,
        "stock": 24,
        "unit": "pcs",
        "active": true,
        "created_at": "2026-08-25T09:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 20
  }
  ```

#### `GET /products/{id}`
Mendapatkan rincian detail data satu produk tertentu.
* **Autentikasi:** Bearer Token (Semua Role)
* **Response Sukses (`200 OK`):** Format objek sama persis dengan elemen item di `GET /products`.
* **Expected Errors:**
  * `404 Not Found` — `{"error": "Produk tidak ditemukan."}`

#### `POST /products` 🔒 Admin
Menambahkan produk baru ke dalam katalog toko. Otomatis membuat riwayat mutasi stok awal jika kuantitas stock diset lebih besar dari 0.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "name": "Minyak Goreng 2L",
    "sku": "MNG-002",
    "barcode": "8992345678122",
    "categoryId": "2da1fb9b-00fa-4009-847c-7f5fa9374021",
    "buyPrice": 28000,
    "sellPrice": 32000,
    "stock": 15,
    "unit": "pcs"
  }
  ```
  *(Catatan: `categoryId`, `buyPrice`, `stock`, `barcode`, `unit` bersifat opsional)*
* **Response Sukses (`201 Created`):** Mengembalikan data lengkap produk yang berhasil dibuat (format sama dengan `GET /products/{id}`).
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "Kategori tidak ditemukan di toko Anda."}`
  * `409 Conflict` — `{"error": "SKU sudah digunakan di toko ini."}`

#### `PUT /products/{id}` 🔒 Admin
Memperbarui data atribut spesifikasi produk. **Harap Dicatat:** Stok produk tidak boleh dan tidak dapat diubah melalui endpoint ini (stok hanya bisa dimutasi secara aman lewat transaksi POS atau penyesuaian stok khusus).
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:** sama dengan payload `POST /products` tanpa parameter `stock`.
* **Response Sukses (`200 OK`):** Mengembalikan objek produk terbaru yang sudah diperbarui.
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "Kategori tidak ditemukan di toko Anda."}`
  * `409 Conflict` — `{"error": "SKU sudah digunakan di toko ini."}`

#### `PATCH /products/{id}/active` 🔒 Admin
Mengubah status aktif/nonaktif dari suatu produk. Produk yang dinonaktifkan tidak akan muncul di layar POS penjualan kasir.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "active": false
  }
  ```
* **Response Sukses (`200 OK`):**
  ```json
  {
    "message": "Status produk diperbarui."
  }
  ```

---

### 📈 Stok & Riwayat Mutasi (Inventory) 🔒 Admin

#### `POST /stock/adjustments`
Melakukan penyesuaian/koreksi nilai stok fisik barang di gudang. Seluruh proses penyesuaian stok dan mutasi riwayat berjalan secara atomik dalam database.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "productId": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
    "direction": "minus",
    "qty": 2,
    "reason": "Barang rusak / bocor di rak"
  }
  ```
  *(Catatan: `direction` bernilai `"plus"` untuk menambah stok atau `"minus"` untuk mengurangi stok. `reason` wajib diisi)*
* **Response Sukses (`200 OK`):**
  ```json
  {
    "product": {
      "id": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
      "category_id": "2da1fb9b-00fa-4009-847c-7f5fa9374021",
      "category_name": "Sembako",
      "name": "Beras Premium 5kg",
      "sku": "BR-001",
      "buy_price": 62000,
      "sell_price": 68000,
      "stock": 22,
      "unit": "pcs",
      "active": true,
      "created_at": "..."
    }
  }
  ```
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "arah penyesuaian harus 'plus' atau 'minus'"}`
  * `400 Bad Request` — `{"error": "stok tidak boleh negatif"}` (Terjadi jika pengurangan kuantitas stok melampaui jumlah stok yang ada saat ini).
  * `404 Not Found` — `{"error": "Produk tidak ditemukan."}`

#### `GET /movements`
Mendapatkan riwayat mutasi keluar masuk stok di toko secara kronologis (terbaru dulu).
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Query Parameters:**
  * `type` (string, opsional): Filter jenis mutasi (`"sale"` / `"refund"` / `"adjust"` / `"initial"`).
  * `productId` (uuid, opsional): Memfilter riwayat mutasi produk tertentu saja.
  * `page` (int, opsional): Halaman keberapa (default: `1`).
  * `limit` (int, opsional): Batas item per halaman (default: `25`).
* **Response Sukses (`200 OK`):**
  ```json
  {
    "items": [
      {
        "id": "6fa872b2-6523-4e00-9ff9-78fffa72da72",
        "product_id": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
        "product_name": "Beras Premium 5kg",
        "type": "adjust",
        "qty": -2,
        "reason": "Barang rusak / bocor di rak",
        "actor": "Bu Sari",
        "created_at": "2026-08-27T10:50:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 25
  }
  ```

---

### 🛒 Transaksi Penjualan (Transactions)

#### `POST /transactions`
Menyelesaikan proses transaksi penjualan POS belanja (Checkout). Stok barang otomatis dikurangi dan mutasi penjualan terekam secara real-time.
* **Autentikasi:** Bearer Token (Semua Role)
* **Request Expected:**
  ```json
  {
    "items": [
      {
        "productId": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
        "qty": 2
      }
    ],
    "discount": 5000,
    "method": "Cash",
    "paid": 150000,
    "customer": "Budi Santoso"
  }
  ```
  *(Catatan: Atribut `discount`, `paid`, `customer` bersifat opsional. Jika metode pembayaran adalah `"Cash"`, nominal `paid` wajib diisi dan harus bernilai lebih besar atau sama dengan total belanja bersih)*
  
  **Pilihan Metode Pembayaran (`method`):**  
  `"Cash"`, `"Bank Transfer"`, `"QRIS"`, `"E-Wallet"`, `"Card"`.
* **Response Sukses (`201 Created`):**
  ```json
  {
    "id": "TRX-0001",
    "seq": 1,
    "cashier_name": "Andi Kasir",
    "items": [
      {
        "product_id": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
        "name": "Beras Premium 5kg",
        "buy_price": 62000,
        "price": 68000,
        "qty": 2
      }
    ],
    "subtotal": 136000,
    "discount": 5000,
    "tax": 13100,
    "total": 144100,
    "method": "Cash",
    "paid": 150000,
    "change": 5900,
    "status": "completed",
    "customer": "Budi Santoso",
    "time": "2026-08-27T11:00:00Z"
  }
  ```
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "Keranjang kosong."}`
  * `400 Bad Request` — `{"error": "Jumlah bayar kurang dari total."}`
  * `400 Bad Request` — `{"error": "Diskon melebihi subtotal."}`
  * `400 Bad Request` — `{"error": "Ada produk yang tidak aktif."}`
  * `409 Conflict` — `{"error": "Stok tidak cukup untuk menyelesaikan transaksi."}`

#### `GET /transactions`
Mendapatkan daftar data riwayat transaksi penjualan.
* **Autentikasi:** Bearer Token (Semua Role)
* **Penyaringan Otomatis (Penting!):** Jika login sebagai Kasir (`cashier`), endpoint ini otomatis hanya akan memunculkan daftar transaksi yang diproses oleh kasir tersebut saja. Admin toko dapat melihat semua data transaksi staff manapun.
* **Query Parameters:**
  * `q` (string, opsional): Pencarian berbasis teks kecocokan ID transaksi atau nama kasir.
  * `method` (string, opsional): Filter cara pembayaran (misal: `"QRIS"`).
  * `date` (string, opsional): Filter transaksi per tanggal format `YYYY-MM-DD`.
  * `page` (int, opsional): Halaman keberapa (default: `1`).
  * `limit` (int, opsional): Batas item per halaman (default: `20`).
* **Response Sukses (`200 OK`):**
  ```json
  {
    "items": [
      {
        "id": "TRX-0001",
        "seq": 1,
        "cashier_name": "Andi Kasir",
        "items": [ ... ],
        "subtotal": 136000,
        "discount": 5000,
        "tax": 13100,
        "total": 144100,
        "method": "Cash",
        "paid": 150000,
        "change": 5900,
        "status": "completed",
        "customer": "Budi Santoso",
        "time": "2026-08-27T11:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 20
  }
  ```

#### `GET /transactions/{id}`
Mendapatkan detail struk informasi dari satu transaksi penjualan berdasarkan ID.
* **Autentikasi:** Bearer Token (Semua Role)
* **Response Sukses (`200 OK`):** Format objek sama dengan respons transaksi.
* **Expected Errors:**
  * `404 Not Found` — `{"error": "Transaksi tidak ditemukan."}` (Jika kasir mencoba mengakses struk transaksi milik staff kasir lain).

#### `POST /transactions/{id}/refund` 🔒 Admin
Memproses pengembalian barang (Refund) sebagian atau seluruhnya dari suatu transaksi penjualan yang telah sukses (`completed`). Otomatis mengembalikan stok produk yang di-refund dan mencatat mutasi masuk ke histori pergudangan.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:**
  ```json
  {
    "items": [
      {
        "productId": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
        "qty": 1
      }
    ],
    "reason": "Salah beli ukuran / varian"
  }
  ```
* **Response Sukses (`200 OK`):** Mengembalikan data terbaru transaksi tersebut setelah pemutakhiran status refund (Status transaksi otomatis berubah menjadi `"refunded"` jika seluruh kuantitas item dalam transaksi dikembalikan sepenuhnya).
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "Qty refund melebihi jumlah terjual."}`
  * `400 Bad Request` — `{"error": "alasan refund wajib diisi"}`
  * `409 Conflict` — `{"error": "Transaksi ini tidak dapat direfund."}` (Bukan berstatus `"completed"`).
  * `404 Not Found` — `{"error": "Transaksi tidak ditemukan."}`

---

### ⚙️ Pengaturan Toko (Store Settings)

#### `GET /settings`
Mengambil konfigurasi informasi identitas fisik toko, format struk, dan struktur perpajakan.
* **Autentikasi:** Bearer Token (Semua Role)
* **Response Sukses (`200 OK`):**
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

#### `PUT /settings` 🔒 Admin
Memperbarui konfigurasi parameter data toko dan struk.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Request Expected:** Struktur payload JSON sama persis dengan response `GET /settings`.
* **Response Sukses (`200 OK`):** Mengembalikan data terbaru hasil konfigurasi yang telah diupdate.
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "nama toko wajib diisi"}`
  * `400 Bad Request` — `{"error": "Zona waktu tidak valid."}` (Misal: format timezone tidak dikenali).

---

### 📊 Ringkasan Dashboard (Dashboard)

#### `GET /dashboard`
Mendapatkan laporan matriks ringkas harian yang disesuaikan secara dinamis berdasarkan role akun login (*Role-Aware Dashboard*).
* **Autentikasi:** Bearer Token (Semua Role)

* **Response Sukses Admin (`200 OK`):**
  ```json
  {
    "role": "admin",
    "today": {
      "omzet": 1500000,
      "trx_count": 12,
      "items_sold": 34,
      "low_stock": 3
    },
    "sales7": [
      { "date": "2026-08-21", "omzet": 1200000 },
      { "date": "2026-08-22", "omzet": 1400000 },
      { "date": "2026-08-23", "omzet": 1000000 },
      { "date": "2026-08-24", "omzet": 1100000 },
      { "date": "2026-08-25", "omzet": 1300000 },
      { "date": "2026-08-26", "omzet": 950000 },
      { "date": "2026-08-27", "omzet": 1500000 }
    ],
    "methods": [
      { "method": "Cash", "total": 800000 },
      { "method": "QRIS", "total": 700000 }
    ],
    "top_products": [
      {
        "product_id": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
        "name": "Beras Premium 5kg",
        "qty": 10,
        "revenue": 680000
      }
    ],
    "recent": [
      {
        "id": "TRX-0012",
        "cashier_name": "Andi Kasir",
        "total": 131000,
        "status": "completed",
        "time": "2026-08-27T10:30:00Z"
      }
    ]
  }
  ```

* **Response Sukses Kasir (`200 OK`):**
  ```json
  {
    "role": "cashier",
    "today": {
      "omzet": 500000,
      "trx_count": 5,
      "items_sold": 12
    },
    "recent": [
      {
        "id": "TRX-0010",
        "cashier_name": "Andi Kasir",
        "total": 50000,
        "status": "completed",
        "time": "2026-08-27T09:45:00Z"
      }
    ]
  }
  ```

---

### 📝 Penarikan Laporan Penjualan (Reports) 🔒 Admin

#### `GET /reports`
Menarik sekumpulan bundle laporan terperinci toko berdasarkan pilihan filter periode waktu tertentu. Sangat berguna untuk kebutuhan pencetakan laporan dan grafik rekapitulasi performa.
* **Autentikasi:** Bearer Token (Hanya Admin)
* **Query Parameters:**
  * `period` (string, opsional): Batasan waktu penarikan laporan.  
    **Pilihan Periode:** `"today"`, `"yesterday"`, `"week"`, `"month"`, `"all"` (default: `"all"` / `""`).
* **Response Sukses (`200 OK`):**
  ```json
  {
    "period": "today",
    "summary": {
      "omzet": 1500000,
      "trx_count": 12,
      "items_sold": 34,
      "gross_profit": 300000
    },
    "by_method": [
      { "method": "Cash", "total": 800000 },
      { "method": "QRIS", "total": 700000 }
    ],
    "by_status": [
      { "status": "completed", "count": 10 },
      { "status": "refunded", "count": 2 }
    ],
    "products": [
      {
        "product_id": "761da11a-1f0b-4d99-897d-6fffa21da1a0",
        "name": "Beras Premium 5kg",
        "sku": "BR-001",
        "qty": 10,
        "revenue": 680000,
        "profit": 60000
      }
    ],
    "transactions": [
      {
        "date": "2026-08-27",
        "id": "TRX-0001",
        "cashier": "Andi Kasir",
        "method": "Cash",
        "total": 131000,
        "hpp": 124000,
        "profit": 7000,
        "status": "completed"
      }
    ],
    "stock": [
      {
        "name": "Beras Premium 5kg",
        "sku": "BR-001",
        "stock": 24,
        "buy_price": 62000,
        "sell_price": 68000,
        "stock_value": 1488000
      }
    ]
  }
  ```
* **Expected Errors:**
  * `400 Bad Request` — `{"error": "Periode tidak valid."}`

---

## ⚙️ Variabel Lingkungan (Environment Variables)

Isi file konfigurasi `.env` sebelum menjalankan aplikasi:

| Nama Variabel | Wajib | Nilai Default | Penjelasan |
|---|---|---|---|
| `PORT` | Tidak | `8080` | Port HTTP lokal untuk server Chi. |
| `DATABASE_URL` | **Ya** | — | Connection string PostgreSQL (`postgres://user:pass@host:port/db`). |
| `JWT_SECRET` | **Ya** | — | Kunci enkripsi rahasia penandatanganan token JWT HS256. |
| `ACCESS_TTL_MINUTES` | Tidak | `15` | Masa kadaluwarsa Access Token (Menit). |
| `REFRESH_TTL_DAYS` | Tidak | `7` | Masa kadaluwarsa Refresh Token (Hari). |
| `CORS_ORIGINS` | Tidak | `http://localhost:5173` | Daftar asal URL frontend yang diijinkan (pisah koma). |

---

## 🚀 Cara Menjalankan Server Lokal

1. Salin konfigurasi environment default:
   ```bash
   cp .env.example .env
   ```
2. Sesuaikan isi nilai `.env` dengan kredensial database lokal Anda.
3. Jalankan migrasi database dan jalankan server web:
   ```bash
   make run
   ```
4. Server REST API Anda kini siap diakses pada alamat lokal `http://localhost:8080/api/v1`.
