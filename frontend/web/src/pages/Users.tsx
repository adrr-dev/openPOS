import { useEffect, useState } from 'react'
import { ApiError, apiCreateUser, apiListUsers, apiSetUserActive, type BackendUser } from '../lib/api'
import { fmtDate } from '../lib/store'
import { Button, Input, Modal, PageHead, Pill, Td, Th } from '../lib/ui'

export default function Users() {
  const [users, setUsers] = useState<BackendUser[] | null>(null)
  const [err, setErr] = useState('')
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [formErr, setFormErr] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    apiListUsers().then(setUsers).catch((e) => setErr(e instanceof Error ? e.message : 'Gagal memuat akun.'))
  }, [])

  async function reload() {
    try {
      setUsers(await apiListUsers())
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Gagal memuat akun.')
    }
  }

  async function create() {
    setFormErr('')
    if (!name.trim()) return setFormErr('Nama wajib diisi.')
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) return setFormErr('Masukkan email yang valid.')
    if (password.length < 8) return setFormErr('Kata sandi minimal 8 karakter.')
    setBusy(true)
    try {
      await apiCreateUser(name.trim(), email.trim().toLowerCase(), password)
      setOpen(false); setName(''); setEmail(''); setPassword('')
      await reload()
    } catch (ex) {
      setFormErr(ex instanceof ApiError ? ex.message : 'Gagal membuat akun.')
    } finally {
      setBusy(false)
    }
  }

  async function toggle(u: BackendUser) {
    setErr('')
    try {
      await apiSetUserActive(u.id, !u.active)
      await reload()
    } catch (ex) {
      setErr(ex instanceof ApiError ? ex.message : 'Gagal mengubah status akun.')
    }
  }

  return (
    <>
      <PageHead
        title="User Management"
        sub={users ? `Akun toko Anda · ${users.length} akun` : 'Memuat…'}
        right={<Button onClick={() => { setOpen(true); setFormErr('') }}>+ Tambah Kasir</Button>}
      />

      {err && <p className="mb-4 rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{err}</p>}

      <div className="overflow-x-auto rounded-2xl bg-cream p-2">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <Th>Nama</Th><Th>Email</Th><Th>Role</Th><Th>Status</Th><Th>Bergabung</Th><Th />
            </tr>
          </thead>
          <tbody>
            {users?.length === 0 && (
              <tr><td colSpan={6} className="py-10 text-center text-sm text-fog">Belum ada kasir — tambahkan akun kasir pertama untuk toko Anda.</td></tr>
            )}
            {(users ?? []).map((u) => (
              <tr key={u.id}>
                <Td><span className="font-medium text-fg">{u.name}</span></Td>
                <Td mono>{u.email}</Td>
                <Td><Pill tone={u.role === 'admin' ? 'ok' : 'muted'}>{u.role === 'admin' ? 'Admin' : 'Kasir'}</Pill></Td>
                <Td><Pill tone={u.active ? 'ok' : 'warn'}>{u.active ? 'Aktif' : 'Nonaktif'}</Pill></Td>
                <Td mono>{u.created_at ? fmtDate(u.created_at) : '—'}</Td>
                <Td>
                  {u.role === 'cashier' && (
                    <button className="text-[13px] text-muted hover:underline" onClick={() => toggle(u)}>
                      {u.active ? 'Nonaktifkan' : 'Aktifkan'}
                    </button>
                  )}
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Modal open={open} title="Tambah Kasir" onClose={() => setOpen(false)}>
        <div className="space-y-4">
          {formErr && <p className="rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{formErr}</p>}
          <Input label="Nama" value={name} onChange={setName} placeholder="Nama kasir" />
          <Input label="Email" type="email" value={email} onChange={setEmail} placeholder="kasir@tokosaya.com" />
          <Input label="Kata sandi" type="password" value={password} onChange={setPassword} placeholder="Minimal 8 karakter" />
          <Button className="w-full" disabled={busy} onClick={create}>{busy ? 'Membuat…' : 'Buat Akun Kasir'}</Button>
        </div>
      </Modal>
    </>
  )
}
