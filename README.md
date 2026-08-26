# openPOS — Backend

Backend REST untuk OpenPOS — sistem kasir gratis untuk UMKM Indonesia.
Stack: **Go (chi) → PostgreSQL (Supabase)**, terdeploy di **Vercel**.

## 🌐 API Produksi (live)

```
https://openpos-api.vercel.app/api/v1
```

Cek kesehatan: <https://openpos-api.vercel.app/api/v1/health>

> **Mengembangkan frontend? Anda TIDAK perlu menjalankan backend lokal.**
> Arahkan `VITE_API_URL` ke URL produksi di atas — detail lengkap ada di [`EXPECTED.md`](./EXPECTED.md).

## Menjalankan Backend Secara Lokal

File `.env` berisi rahasia koneksi database & JWT — **minta nilainya langsung ke maintainer proyek**.
Struktur variabel tersedia di [`.env.example`](./.env.example).

```bash
cp .env.example .env    # isi nilai riil (hubungi maintainer)
make run                # http://localhost:8080/api/v1
```

| Target | Fungsi |
|---|---|
| `make install` | `go mod tidy` |
| `make run` | server dev |
| `make build` / `make start` | compile → `bin/openpos-api` |
| `make check` | vet + build semua paket |
| `make test` | seluruh test (muat `.env`) |

Migrasi skema berjalan otomatis saat startup.

## Dokumen

| File | Isi |
|---|---|
| [`PROJECT.md`](./PROJECT.md) | Kontrak API lengkap, skema DB, konfigurasi env, deploy Vercel |
| [`EXPECTED.md`](./EXPECTED.md) | Handoff tim frontend + checklist integrasi tiap fase |

Repo: <https://github.com/0xMinomus/openPOS>
Frontend produksi: dikelola tim frontend (`VITE_API_URL` → URL di atas).
