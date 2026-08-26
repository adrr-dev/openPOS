import { useCallback, useEffect, useState } from 'react'
import {
  apiAdjustStock, apiListMovements, apiListProducts,
  type ApiMovement, type ApiProduct, type MovementType, ApiError,
} from '../lib/api'
import { fmtDate, fmtTime } from '../lib/store'
import { Button, Input, Modal, PageHead, Pill, Td, Th } from '../lib/ui'

const TYPE_LABEL: Record<MovementType, string> = {
  sale: 'Penjualan', refund: 'Refund', adjust: 'Penyesuaian', initial: 'Stok awal',
}
const MOV_PAGE = 25

export default function Stok() {
  const [products, setProducts] = useState<ApiProduct[] | null>(null)
  const [movements, setMovements] = useState<ApiMovement[] | null>(null)
  const [movTotal, setMovTotal] = useState(0)
  const [movPage, setMovPage] = useState(1)

  const [adjustFor, setAdjustFor] = useState<ApiProduct | null>(null)
  const [qty, setQty] = useState('')
  const [reason, setReason] = useState('')
  const [type, setType] = useState<'plus' | 'minus'>('plus')
  const [formErr, setFormErr] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const msg = (e: unknown) => (e instanceof ApiError ? e.message : 'Terjadi kesalahan. Coba lagi.')

  const loadProducts = useCallback(async () => {
    try {
      // ambil semua halaman (dataset UMKM kecil; server dibatasi 200/halaman)
      const out: ApiProduct[] = []
      let p = 1
      for (;;) {
        const d = await apiListProducts({ page: p, limit: 200 })
        out.push(...d.items)
        if (out.length >= d.total || d.items.length === 0) break
        p++
      }
      setProducts(out)
    } catch (e) { setErr(msg(e)) }
  }, [])

  const loadMovements = useCallback(async (p: number) => {
    try {
      const d = await apiListMovements({ page: p, limit: MOV_PAGE })
      setMovements(d.items); setMovTotal(d.total); setMovPage(d.page)
    } catch (e) { setErr(msg(e)) }
  }, [])

  useEffect(() => { loadProducts() }, [loadProducts])
  useEffect(() => { loadMovements(1) }, [loadMovements])

  async function submitAdjust() {
    if (!adjustFor) return
    const n = Number(qty)
    const r = reason.trim()
    if (!Number.isFinite(n) || n < 1) return setFormErr('Jumlah harus minimal 1.')
    if (!r) return setFormErr('Alasan penyesuaian wajib diisi.')
    setBusy(true); setFormErr('')
    try {
      await apiAdjustStock(adjustFor.id, type, n, r)
      setAdjustFor(null); setQty(''); setReason(''); setType('plus')
      await Promise.all([loadProducts(), loadMovements(1)])
    } catch (e) {
      setFormErr(msg(e))
    } finally { setBusy(false) }
  }

  return (
    <>
      <PageHead title="Stok" sub="Status stok saat ini dan riwayat pergerakan barang." />
      {err && <p className="mb-4 rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{err}</p>}

      <div className="grid gap-4 lg:grid-cols-2">
        <div>
          <h2 className="mb-3 font-mono text-xs uppercase tracking-wider text-fog">Status stok</h2>
          <div className="overflow-x-auto rounded-2xl bg-cream p-2">
            <table className="w-full border-collapse">
              <thead>
                <tr>
                  <Th>Produk</Th><Th right>Stok</Th><Th>Status</Th><Th />
                </tr>
              </thead>
              <tbody>
                {(products ?? []).filter((p) => p.active).map((p) => (
                  <tr key={p.id}>
                    <Td><span className="font-medium text-fg">{p.name}</span></Td>
                    <Td right><span className={p.stock <= 5 ? 'font-medium text-ember' : ''}>{p.stock} {p.unit}</span></Td>
                    <Td>
                      {p.stock === 0 ? <Pill tone="warn">Habis</Pill> : p.stock <= 5 ? <Pill tone="warn">Menipis</Pill> : <Pill>Aman</Pill>}
                    </Td>
                    <Td>
                      <button className="text-[13px] font-medium text-jet hover:underline" onClick={() => { setAdjustFor(p); setFormErr('') }}>Penyesuaian</button>
                    </Td>
                  </tr>
                ))}
                {products !== null && products.filter((p) => p.active).length === 0 && (
                  <tr><td colSpan={4} className="py-10 text-center text-sm text-fog">Belum ada produk aktif.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>

        <div>
          <h2 className="mb-3 font-mono text-xs uppercase tracking-wider text-fog">
            Riwayat pergerakan{movTotal > 0 ? ` · ${movTotal}` : ''}
          </h2>
          <div className="max-h-[70vh] overflow-y-auto rounded-2xl bg-cream p-2">
            {movements !== null && movements.length === 0 ? (
              <p className="py-10 text-center text-sm text-fog">Belum ada pergerakan stok.</p>
            ) : (
              <table className="w-full border-collapse">
                <thead>
                  <tr>
                    <Th>Waktu</Th><Th>Produk</Th><Th>Jenis</Th><Th right>Qty</Th><Th>Alasan</Th><Th>Aktor</Th>
                  </tr>
                </thead>
                <tbody>
                  {(movements ?? []).map((m) => (
                    <tr key={m.id}>
                      <Td mono>{m.created_at ? `${fmtDate(m.created_at)} ${fmtTime(m.created_at)}` : '—'}</Td>
                      <Td>{m.product_name ?? '—'}</Td>
                      <Td><Pill tone={m.type === 'sale' ? 'ok' : m.type === 'refund' || m.type === 'initial' ? 'warn' : 'muted'}>{TYPE_LABEL[m.type]}</Pill></Td>
                      <Td right><span className={m.qty > 0 ? 'text-sprout' : 'text-ember'}>{m.qty > 0 ? '+' : ''}{m.qty}</span></Td>
                      <Td>{m.reason}</Td>
                      <Td>{m.actor}</Td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
          {movTotal > MOV_PAGE && (
            <div className="mt-3 flex items-center justify-between text-[13px] text-muted">
              <Button variant="ghost" disabled={movPage <= 1} onClick={() => loadMovements(movPage - 1)}>← Sebelumnya</Button>
              <span className="font-mono text-xs text-fog">halaman {movPage}</span>
              <Button variant="ghost" disabled={movPage * MOV_PAGE >= movTotal} onClick={() => loadMovements(movPage + 1)}>Berikutnya →</Button>
            </div>
          )}
        </div>
      </div>

      <Modal open={!!adjustFor} title={`Penyesuaian stok — ${adjustFor?.name ?? ''}`} onClose={() => setAdjustFor(null)}>
        {adjustFor && (
          <div className="space-y-4">
            {formErr && <p className="rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{formErr}</p>}
            <p className="text-sm text-muted">Stok saat ini: <strong className="text-fg">{adjustFor.stock} {adjustFor.unit}</strong></p>
            <div className="flex gap-2">
              <button
                onClick={() => setType('plus')}
                className={`flex-1 rounded-full border py-2 text-sm ${type === 'plus' ? 'border-jet bg-jet text-paper' : 'border-dove text-muted'}`}
              >
                + Tambah
              </button>
              <button
                onClick={() => setType('minus')}
                className={`flex-1 rounded-full border py-2 text-sm ${type === 'minus' ? 'border-jet bg-jet text-paper' : 'border-dove text-muted'}`}
              >
                − Kurangi
              </button>
            </div>
            <Input label="Jumlah" type="number" value={qty} onChange={setQty} placeholder="0" />
            <Input label="Alasan (wajib)" value={reason} onChange={setReason} placeholder="cth: barang rusak, stok fisik berbeda" />
            <Button className="w-full" disabled={busy || !Number(qty) || !reason.trim()} onClick={submitAdjust}>
              {busy ? 'Menyimpan…' : 'Simpan Penyesuaian'}
            </Button>
          </div>
        )}
      </Modal>
    </>
  )
}
