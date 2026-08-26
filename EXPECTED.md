# EXPECTED.md — Panduan Integrasi Frontend ↔ API OpenPOS

> Untuk pemegang **repo frontend bersih**: <https://github.com/0xMinomus/openPOS>
> (versi localStorage — `store.ts`, seed `admin/123`, semua halaman baca `db.*`).
> Tujuan dokumen ini: daftar **perubahan yang harus Anda edit** agar aplikasi bekerja
> dengan API backend, tanpa menjalankan backend lokal sama sekali.

---

## 0. Informasi dasar

| Item | Nilai |
|---|---|
| Base URL API (produksi) | `https://openpos-api.vercel.app/api/v1` |
| Health check | `GET /health` → `{"database":"up",...}` |
| Kontrak lengkap endpoint | [`PROJECT.md`](./PROJECT.md) |
| Menjalankan backend lokal | **Tidak diperlukan** untuk mengembangkan frontend |

Aturan main:
1. Base URL dibaca dari env: `VITE_API_URL` (prod) — jangan hardcode.
2. Semua error server berupa JSON `{ "error": "pesan Indonesia siap-tampil" }` → langsung tampilkan.
3. Access token kedaluwarsa (15 menit) → tangani lewat auto-refresh (lihat Langkah 1).

## Peta besar: apa yang berubah

| Sisi | Sebelum (repo bersih) | Sesudah integrasi |
|---|---|---|
| Autentikasi | `store.ts` cek password lokal, seed `admin/123` | `POST /auth/login|register`, JWT + refresh token |
| Data bisnis | array di `localStorage` (`op_db_v1`) | endpoint REST (produk, stok, transaksi, users, laporan) |
| RBAC | disembunyikan di menu | **ditegakkan server** (kasir → 403) |
| Ganti akun cepat | `loginAs()` tanpa sandi | dihapus — logout + login penuh |

Halaman yang **tidak perlu diubah struktur visualnya**: semuanya. Yang berubah hanya *sumber datanya*.

---

## File yang diedit — basis bersih → hasil integrasi

Dua opsi pengerjaan. **Rekomendasi: Opsi A** bila ingin cepat & hasil identik dengan yang sudah teruji;
**Opsi B** bila ingin memahami dan menulis sendiri setiap perubahan.

### Opsi A — Salin file jadi dari fork integrasi (Adrr)

Timpa file-file berikut pada repo bersih Anda (path relatif terhadap `web/`), lalu set env:

```
src/lib/api.ts        ← FILE BARU (fondasi semua request)
src/lib/store.ts      ← auth via API + hapus seed demo/loginAs
vite.config.ts        ← tambah proxy dev /api → localhost:8080
src/pages/Masuk.tsx       src/pages/Users.tsx
src/pages/Daftar.tsx      src/pages/Produk.tsx
src/pages/AppShell.tsx    src/pages/Stok.tsx
src/pages/Pos.tsx         src/pages/Transaksi.tsx
src/pages/Dashboard.tsx   src/pages/Laporan.tsx
                          src/pages/Pengaturan.tsx
```

Tidak tersentuh: `Landing`, `Navbar`, `components/ui/*`, `hooks`, `index.css`, `App.tsx`.
Sumber salinan: minta arsip/repo fork integrasi ke Adrr.

### Opsi B — Edit manual dari basis bersih

Ikuti **Langkah 1 → 5g** di bawah secara berurutan. Perkiraan bobot per file agar bisa dibagi tugas:

| Bobot | File |
|---|---|
| Besar | `lib/api.ts` (baru) · `Pos.tsx` · `Transaksi.tsx` · `Produk.tsx` · `store.ts` |
| Sedang | `Stok.tsx` · `Dashboard.tsx` · `Laporan.tsx` · `Pengaturan.tsx` · `Users.tsx` |
| Ringan | `Masuk.tsx` · `Daftar.tsx` · `AppShell.tsx` · `vite.config.ts` |

### `vercel.json` frontend — sudah beres di repo bersih ✅

Isinya sudah tepat (`framework: vite`, build `npm run build`, output `dist`, rewrites SPA).
Cukup tambah Environment Variable `VITE_API_URL=https://openpos-api.vercel.app/api/v1`.

---

## Langkah 1 — Buat `src/lib/api.ts` (FILE BARU — fondasi seluruh integrasi)

Satu file yang menangani: base URL, penyimpanan token, auto-refresh, dan semua helper endpoint.

Tanggung jawab wajib:
- `BASE = import.meta.env.VITE_API_URL ?? '/api/v1'`
- Token disimpan di `localStorage`: kunci `op_access` & `op_refresh`
- Setiap request auth menyertakan header `Authorization: Bearer <access>`; bila respon 401
  → panggil `POST /auth/refresh` **sekali**, simpan pasangan token baru, ulangi request;
  gagal → sesi habis (arahkan ke `/masuk`)
- Error dilempar sebagai `ApiError extends Error` dengan `{ status, message, code }`;
  `code = 'passcode_required'` bila pesan server persis `"passcode_required"`

Daftar helper yang dipakai halaman-halaman (method · path):

| Helper | HTTP | Dipakai halaman |
|---|---|---|
| `apiLogin(email,password,passcode?)` | POST `/auth/login` | Masuk |
| `apiRegister(name,email,password,storeName)` | POST `/auth/register` | Daftar |
| `apiLogout()` | POST `/auth/logout` | AppShell |
| `apiMe()` | GET `/auth/me` | bootstrap sesi |
| `apiListUsers / apiCreateUser / apiSetUserActive` | GET/POST `/users`, PATCH `/users/{id}/active` | Users |
| `apiListCategories / apiCreateCategory / apiDeleteCategory` | `/categories` | Produk |
| `apiListProducts({q,categoryId,active,page,limit}) / apiCreateProduct / apiUpdateProduct / apiSetProductActive` | `/products…` | Produk, Stok, Pos |
| `apiListMovements({type,productId,page}) / apiAdjustStock(productId,direction,qty,reason)` | `/movements`, `/stock/adjustments` | Stok |
| `apiCheckout({items:[{productId,qty}],discount,method,paid})` | POST `/transactions` | Pos |
| `apiListTransactions({q,method,date,page}) / apiRefundTransaction(id,items,reason)` | `/transactions…` | Transaksi |
| `apiGetSettings / apiUpdateSettings / apiSetPasscode(userId,passcode)` | `/settings`, `/users/{id}/passcode` | Pengaturan |
| `apiGetDashboard()` · `apiGetReport(period)` | `/dashboard`, `/reports?period=` | Dashboard, Laporan |

Bentuk data penting (snake_case dari server):
- Produk: `{ id, category_id, category_name, name, sku, barcode, buy_price, sell_price, stock, unit, active }`
- Kategori: `{ id, name, active }`
- Transaksi: `{ id:"TRX-0001", seq, cashier_name, items:[{product_id,name,buy_price,price,qty}], subtotal, discount, tax, total, method, paid, change, status, time }`
- Movement: `{ id, product_id, product_name, type:"sale|refund|adjust|initial", qty(±), reason, actor, created_at }`

> Implementasi lengkap `api.ts` (±300 baris) sudah jadi — **minta filenya ke tim backend**
> atau salin dari folder `frontend/web/src/lib/api.ts` pada fork integrasi. Dokumen ini cukup
> untuk memahami & mereplikasi manual.

## Langkah 2 — Proxy dev (`vite.config.ts`)

```ts
server: { proxy: { '/api': 'http://localhost:8080' } }
```
Hanya untuk pengembangan. Produksi memakai `VITE_API_URL` penuh.

## Langkah 3 — `src/lib/store.ts` (auth pindah ke server)

| Fungsi lama | Ubah menjadi |
|---|---|
| `login()` sinkron cek `db.accounts` | **async** → `POST /auth/login`; sukses tulis `db.session` + token tersimpan oleh `api.ts`; error dilempar (kode `passcode_required` = tampilkan form passcode, ulangi dengan argumen `passcode`) |
| `register()` tulis akun lokal | async → `POST /auth/register`; cukup set `session` + `settings.storeName` lokal; **jangan** menyalin akun ke `db.accounts` |
| `logout()` | async → `POST /auth/logout` (best-effort) + hapus token + `session=null` |
| `loginAs()` | **HAPUS** (ganti akun tanpa sandi tidak diizinkan backend) |

Tambah/perbaiki:
- Hapus seed akun demo (`admin/123`) dari `seed()` dan blok re-add di `load()`; kunci storage naik ke `op_db_v2`.
- Saat `load()`: paksa `db.accounts = {}` (cermin akun lama membuat halaman Users tampil salah).
- Bootstrap: jika token ada tapi `db.session` kosong → `GET /auth/me` untuk hidrasi; gagal → hapus token.

## Langkah 4 — Halaman auth

- `Masuk.tsx`: hapus semua pengecekan `db.accounts[...]` (validasi kepemilikan akun = tugas server);
  handler jadi `async`; tangkap `ApiError` — `code==='passcode_required'` → tampilkan form passcode.
- `Daftar.tsx`: hapus cek email-duplikat lokal (server balas 409); submit async.
- `AppShell.tsx` (UserMenu): hapus daftar "ganti akun" & alur `loginAs` — sisakan avatar/nama/role + tombol Keluar.

## Langkah 5 — Modul bisnis (kerjakan berurutan)

### 5a. User Management → `Users.tsx`
Endpoint 🔒admin: `GET /users` · `POST /users {name,email,password}` · `PATCH /users/{id}/active {active}`.
- Daftar akun dari `GET /users` (sudah otomatis terbatas toko admin); tombol aktif/nonaktif hanya untuk `role==='cashier'`.

### 5b. Produk + Kategori → `Produk.tsx`
- List: `GET /products?q=&categoryId=&active=&page=&limit=` (search & pagination server-side).
- Create/Update: `POST /products` (stok awal hanya saat create) · `PUT /products/{id}` (**tidak menyentuh stok**) · toggle `PATCH …/active`.
- Kategori: `POST /categories` · `DELETE /categories/{id}` — respons `{soft_deleted:true}` artinya masih dipakai produk (tampilkan info, histori tetap aman).
- SKU duplikat → 409 `"SKU sudah digunakan di toko ini."`. Import CSV: commit baris-per-baris via `POST /products`, laporkan per baris. Export CSV: tarik semua halaman (`limit=200`) lalu susun CSV di klien.

### 5c. Stok → `Stok.tsx`
- Status stok = produk dari `GET /products` (ambil semua halaman).
- Penyesuaian: `POST /stock/adjustments {productId, direction:'plus'|'minus', qty, reason}` — error `"stok tidak boleh negatif"` tampilkan di modal.
- Riwayat: `GET /movements?page=` (terbaru dulu).

### 5d. POS → `Pos.tsx`
- Katalog: `GET /products?active=true` (semua halaman); **stok efektif = stock − jumlah di keranjang**.
- Checkout: `POST /transactions` dengan **hanya** `{items:[{productId,qty}], discount?, method, paid?, customer?}` — subtotal/diskon-validasi/pajak/total/kembalian dihitung server; respons = objek transaksi lengkap → render struk darinya, lalu muat ulang katalog.
- Header/footer/paper/nama toko struk: dari `GET /settings`.

### 5e. Transaksi → `Transaksi.tsx`
- List: `GET /transactions?q=&method=&date=&page=` — kasir otomatis hanya melihat miliknya (jangan filter role di klien).
- Refund (tampilkan tombol hanya untuk admin): `POST /transactions/{id}/refund {items:[{productId,qty}], reason}` — parsial boleh; dobel/kelebihan ditolak server (tampilkan pesannya).

### 5f. Dashboard & Laporan → `Dashboard.tsx`, `Laporan.tsx`
- `GET /dashboard` → bentuk berbeda per `role`: admin (KPI hari ini, `sales7`, `methods`, `top_products`, `recent`) vs kasir (ringkasan shift sendiri saja).
- `GET /reports?period=today|yesterday|week|month|all` → komposit `{summary, by_method, by_status, products, transactions(HPP+profit), stock}`; export CSV disusun di klien dari data ini.

### 5g. Pengaturan → `Pengaturan.tsx`
- `GET /settings` · `PUT /settings {storeName,address,phone,taxEnabled,taxPct,receiptHeader,receiptFooter,paper,timezone}` 🔒admin.
- Passcode per akun: `PUT /users/{id}/passcode {passcode:"12345"|""(hapus)}` 🔒admin.
- Hapus semua `mutate()` lokal di halaman ini.

## Gotcha yang sering menjebak

1. **CORS**: setelah frontend online, kirim domainnya ke tim backend agar masuk `CORS_ORIGINS` — tanpa itu browser memblokir request (curl/postman tidak terpengaruh).
2. **`.env` tidak ikut deploy** — variabel produksi di-set via dashboard Vercel, lalu Redeploy.
3. `VITE_*` ditanam saat **build**; ganti nilai = build ulang.
4. Data lama di localStorage biarkan saja — halaman terintegrasi tak membacanya lagi.
5. Transaksi yang dibuat SEBELUM integrasi tidak akan muncul di server.

## Urutan pengerjaan yang disarankan

`api.ts` + proxy → auth (Langkah 3–4) → Users → Produk → Stok → Pos → Transaksi → Dashboard/Laporan/Pengaturan.
Setiap tahap sudah bisa diuji mandiri terhadap API produksi.

---
*Kontrak detail tiap endpoint: [`PROJECT.md`](./PROJECT.md). Pertanyaan/permintaan perubahan kontrak: tim backend.*
