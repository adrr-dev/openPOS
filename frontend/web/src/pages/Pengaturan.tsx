import { useEffect, useState } from 'react'
import {
  apiGetSettings, apiGetDashboard, apiListUsers, apiSetPasscode,
  apiUpdateSettings, type ApiSettings, type BackendUser, ApiError,
} from '../lib/api'
import { Button, Input, PageHead } from '../lib/ui'

export default function Pengaturan() {
  const [form, setForm] = useState<ApiSettings | null>(null)
  const [users, setUsers] = useState<BackendUser[]>([])
  const [passcodes, setPasscodes] = useState<Record<string, string>>({})
  const [msg, setMsg] = useState('')
  const [pcMsg, setPcMsg] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    apiGetSettings().then(setForm).catch((e) => setErr(e.message))
    apiListUsers()
      .then((us) => {
        setUsers(us)
        // passcode tidak pernah dikirim server (hanya hash) — mulai kosong
        setPasscodes(Object.fromEntries(us.map((u) => [u.id, ''])))
      })
      .catch(() => {})
  }, [])

  if (!form) {
    return (
      <>
        <PageHead title="Pengaturan" sub="Konfigurasi toko, struk, dan pajak." />
        {err ? <p className="rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{err}</p> : <p className="py-10 text-center text-sm text-fog">Memuat…</p>}
      </>
    )
  }

  const set = (k: keyof ApiSettings) => (v: string) =>
    setForm((f) => {
      if (!f) return f
      if (k === 'taxPct') return { ...f, taxPct: Number(v) || 0 }
      return { ...f, [k]: v }
    })

  async function save() {
    if (!form) return
    setErr(''); setMsg(''); setBusy(true)
    try {
      const s = await apiUpdateSettings(form)
      setForm(s)
      setMsg('Pengaturan disimpan.')
      // dashboard header memakai nama toko — pastikan sinkron
      apiGetDashboard().catch(() => {})
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : 'Gagal menyimpan pengaturan.')
    } finally { setBusy(false) }
  }

  async function savePasscode(u: BackendUser) {
    const pc = passcodes[u.id] ?? ''
    if (pc && !/^\d{5}$/.test(pc)) return setPcMsg('Passcode harus 5 angka.')
    setPcMsg('')
    try {
      await apiSetPasscode(u.id, pc)
      setPcMsg(pc ? `Passcode ${u.name} disimpan.` : `Passcode ${u.name} dihapus.`)
      setPasscodes((p) => ({ ...p, [u.id]: '' }))
    } catch (e) {
      setPcMsg(e instanceof ApiError ? e.message : 'Gagal menyimpan passcode.')
    }
  }

  return (
    <>
      <PageHead title="Pengaturan" sub="Konfigurasi toko, struk, dan pajak." />
      <div className="max-w-xl space-y-6">
        {err && <p className="rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{err}</p>}

        <section className="rounded-2xl bg-cream p-6">
          <h2 className="mb-4 font-mono text-xs uppercase tracking-wider text-fog">Profil toko</h2>
          <div className="space-y-4">
            <Input label="Nama toko" value={form.storeName} onChange={set('storeName')} />
            <Input label="Alamat" value={form.address} onChange={set('address')} />
            <Input label="Telepon" value={form.phone} onChange={set('phone')} />
            <label className="flex flex-col gap-1.5 text-[13px] font-medium text-steel">
              Timezone
              <select value={form.timezone} onChange={(e) => setForm({ ...form, timezone: e.target.value })} className="rounded-md border border-border bg-paper px-3.5 py-2.5 text-[15px] focus:border-jet focus:outline-none">
                {['Asia/Makassar', 'Asia/Jakarta', 'Asia/Pontianak', 'Asia/Jayapura'].map((tz) => <option key={tz}>{tz}</option>)}
              </select>
            </label>
          </div>
        </section>

        <section className="rounded-2xl bg-cream p-6">
          <h2 className="mb-4 font-mono text-xs uppercase tracking-wider text-fog">Struk</h2>
          <div className="space-y-4">
            <Input label="Header struk" value={form.receiptHeader} onChange={set('receiptHeader')} />
            <Input label="Footer struk" value={form.receiptFooter} onChange={set('receiptFooter')} />
            <label className="flex flex-col gap-1.5 text-[13px] font-medium text-steel">
              Ukuran kertas default
              <select value={form.paper} onChange={(e) => setForm({ ...form, paper: e.target.value })} className="rounded-md border border-border bg-paper px-3.5 py-2.5 text-[15px] focus:border-jet focus:outline-none">
                <option value="58mm">58 mm</option>
                <option value="80mm">80 mm</option>
              </select>
            </label>
          </div>
        </section>

        <section className="rounded-2xl bg-cream p-6">
          <h2 className="mb-2 font-mono text-xs uppercase tracking-wider text-fog">Passcode akun</h2>
          <p className="mb-4 text-[13px] text-muted">Passcode 5 angka diminta saat login. Kosongkan lalu simpan untuk menonaktifkan.</p>
          <div className="space-y-3">
            {users.map((u) => (
              <div key={u.id} className="flex flex-wrap items-center gap-3 rounded-lg border border-dove bg-paper px-3.5 py-2.5">
                <div className="min-w-36 flex-1">
                  <p className="text-sm font-medium text-fg">{u.name}</p>
                  <p className="font-mono text-[11px] text-fog">{u.role === 'admin' ? 'Admin' : 'Kasir'} · {u.email}</p>
                </div>
                <input
                  value={passcodes[u.id] ?? ''}
                  onChange={(e) => setPasscodes({ ...passcodes, [u.id]: e.target.value.replace(/\D/g, '').slice(0, 5) })}
                  inputMode="numeric"
                  placeholder="5 angka"
                  aria-label={`Passcode ${u.name}`}
                  className="w-28 rounded-md border border-border bg-paper px-3 py-2 text-center font-mono tracking-[0.3em] focus:border-jet focus:outline-none"
                />
                <Button variant="ghost" className="!px-4 !py-1.5 text-xs" onClick={() => savePasscode(u)}>Simpan</Button>
              </div>
            ))}
          </div>
          {pcMsg && <p className="mt-3 text-xs text-sprout">{pcMsg}</p>}
        </section>

        <section className="rounded-2xl bg-cream p-6">
          <div className="space-y-4">
            <label className="flex cursor-pointer items-center gap-2.5 text-sm">
              <input type="checkbox" checked={form.taxEnabled} onChange={(e) => setForm({ ...form, taxEnabled: e.target.checked })} className="h-4 w-4 accent-jet" />
              Aktifkan pajak transaksi
            </label>
            {form.taxEnabled && <Input label="Persentase pajak (%)" type="number" value={String(form.taxPct)} onChange={set('taxPct')} />}
          </div>
        </section>

        <section className="rounded-2xl bg-cream p-6">
          <h2 className="mb-2 font-mono text-xs uppercase tracking-wider text-fog">Informasi</h2>
          <p className="text-sm text-muted">
            Currency: <strong className="text-fg">IDR (Rupiah)</strong>. Data tersimpan aman di server OpenPOS.
          </p>
        </section>

        <Button onClick={save} disabled={busy}>{busy ? 'Menyimpan…' : 'Simpan Pengaturan'}</Button>
        {msg && <p className="text-xs text-sprout">{msg}</p>}
      </div>
    </>
  )
}
