# openPOS — Backend

Backend REST untuk OpenPOS (Go + chi + PostgreSQL/Supabase). Fase saat ini: **skeleton + auth**.

## Mulai cepat

```bash
cp .env.example .env    # isi DATABASE_URL (Supabase) & JWT_SECRET
go run ./cmd/api        # http://localhost:8080/api/v1
```

## Dokumen

| File | Isi |
|---|---|
| [`PROJECT.md`](./PROJECT.md) | Kontrak API, skema DB, konfigurasi, roadmap modul |
| [`EXPECTED.md`](./EXPECTED.md) | Panduan untuk tim frontend: apa yang harus diubah agar memakai API |

Repo: <https://github.com/0xMinomus/openPOS>
