# EXPECTED.md — Yang Harus Diubah Frontend

> File ini untuk **tim frontend**. Berisi daftar perubahan yang diharapkan agar aplikasi web
> memakai API backend alih-alih hardcoded/localStorage. Centang ✔ berarti sudah dikerjakan.

## HANDOFF — Deploy Frontend (Vercel)

| Item | Nilai |
|---|---|
| Build Command | `npm run build` |
| Output Directory | `dist` |
| Node | 22.x |
| Env `VITE_API_URL` | `https://openpos-api.vercel.app/api/v1` |

Tambahkan `vercel.json` di root repo frontend:
```json
{ "rewrites": [{ "source": "/(.*)", "destination": "/index.html" }] }
```

Backend produksi sudah live & teruji: <https://openpos-api.vercel.app/api/v1/health>
**Menjalankan backend lokal tidak diperlukan** untuk mengembangkan frontend.
Kontrak endpoint lengkap: [`PROJECT.md`](./PROJECT.md). Setelah frontend online, kirim URL-nya
ke tim backend agar domainnya ditambahkan ke `CORS_ORIGINS` (saat ini backend hanya mengizinkan
localhost — tanpa itu browser akan memblokir request API dari produksi).

## Aturan main integrasi

1. **Base URL API** dibaca dari env, jangan hardcode:
   - Dev: `/api/v1` (di-proxy Vite ke `http://localhost:8080`, lihat `vite.config.ts`)
   - Prod: set `VITE_API_URL=https://openpos-api.vercel.app/api/v1` di Environment Variables Vercel frontend (URL resmi backend produksi — final)
2. Kontrak lengkap tiap endpoint ada di [`PROJECT.md`](./PROJECT.md).
3. Error dari server memakai pesan Indonesia siap-tampil — langsung tampilkan ke pengguna.
4. Access token kedaluwarsa (default 15 menit) → `apiFetch` otomatis mencoba `POST /auth/refresh` sekali lalu mengulang request. Jika refresh gagal → anggap sesi habis (arahkan ke `/masuk`).

## Status integrasi halaman

| Halaman | Sumber data | Status |
|---|---|---|
| Masuk / Daftar | API auth | ✅ |
| User Management | API `/users` | ✅ |
| Produk | API katalog | ✅ |
| Stok | API stok/movement | ✅ |
| POS Kasir | API katalog + checkout + settings struk | ✅ |
| Transaksi | API transaksi/refund | ✅ |
| Dashboard | API `/dashboard` (role-aware) | ✅ |
| Laporan | API `/reports?period=` | ✅ |
| Pengaturan | API `/settings` + `/users/{id}/passcode` | ✅ |

**Seluruh halaman kini memakai API — tidak ada lagi data bisnis di localStorage.**

## Fase 6 — Dashboard, Laporan, Settings ✅ (selesai)

`GET/PUT /settings` · `PUT /users/{id}/passcode` · `GET /dashboard` (role-aware, zona waktu toko) · `GET /reports?period=today|yesterday|week|month|all` 🔒 admin. Kontrak lengkap di [`PROJECT.md`](./PROJECT.md).

- [x] *(backend)* Settings CRUD + passcode (bcrypt) + dashboard agregat + report komposit
- [x] *(frontend)* `api.ts`: settings/passcode/dashboard/report helpers
- [x] *(frontend)* `Dashboard.tsx`: KPI, grafik 7 hari, metode bayar, top produk, recent; kasir = ringkasan shift
- [x] *(frontend)* `Laporan.tsx`: 5 periode × 4 tab dari satu respons, export CSV per tab
- [x] *(frontend)* `Pengaturan.tsx`: profil toko, struk, pajak via `PUT /settings`; passcode per akun via API
- [x] *(frontend)* `Pos.tsx`: struk membaca header/footer/paper/nama toko dari `GET /settings`

## Fase 1 — Auth ✅

`POST /auth/register|login|refresh|logout` · `GET /auth/me`. Alur passcode dua-langkah via error `passcode_required`. Seed akun demo lokal dihapus; storage key naik `op_db_v2`.

## Fase 2 — Users ✅

`GET /users` · `POST /users` · `PATCH /users/{id}/active` 🔒 admin. `Users.tsx` memakai API penuh. Sisa: bagian passcode di Pengaturan (menyusul).

## Fase 3 — Katalog: Produk & Kategori ✅

`GET/POST /categories`, `DELETE /categories/{id}` (soft bila dipakai), `GET /products` (+filter q/active/page/limit), `POST/PUT/PATCH /products…` (SKU unik per toko; PUT tak menyentuh stok). `Produk.tsx` via API penuh termasuk import/export CSV.

## Fase 4 — Stok & Movement ✅

`GET /movements?type=&productId=&page=` · `POST /stock/adjustments` 🔒 admin (alasan wajib, dilarang negatif, atomik; produk berstok awal dapat movement `initial`). `Stok.tsx` via API penuh.

## Fase 5 — Transaksi & Refund ✅

`POST /transactions` (checkout dihitung server: harga dari DB, pajak dari settings toko, advisory-lock per toko, snapshot item), `GET /transactions?q=&method=&date=` (kasir terbatas miliknya), `GET /transactions/{id}`, `POST /transactions/{id}/refund` 🔒 admin (parsial/penuh; dobel refund ditolak EC-004).

- [x] *(backend)* Migrasi 005: transactions, transaction_items (snapshot), refunds (JSONB), kolom settings di stores
- [x] *(frontend)* `api.ts`: `apiCheckout` / `apiListTransactions` / `apiRefundTransaction`
- [x] *(frontend)* `Pos.tsx`: katalog dari API, stok efektif dikurangi isi keranjang, struk dari respons server
- [x] *(frontend)* `Transaksi.tsx`: list server-side + filter q/method/date, detail dari data list, refund admin via API
- [ ] *(frontend)* `Dashboard.tsx`, `Laporan.tsx`, `Pengaturan.tsx` — menyusul modul #6
