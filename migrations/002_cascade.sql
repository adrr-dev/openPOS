-- FK dengan ON DELETE CASCADE supaya penghapusan manual dari Supabase
-- tetap menjaga konsistensi: hapus store -> users ikut terhapus,
-- hapus user -> refresh_tokens ikut terhapus.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_store_id_fkey;
ALTER TABLE users
    ADD CONSTRAINT users_store_id_fkey
    FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE;
