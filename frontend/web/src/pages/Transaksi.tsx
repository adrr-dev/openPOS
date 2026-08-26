import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  apiListTransactions, apiRefundTransaction,
  type ApiTrx, type ApiTrxItem, ApiError,
} from '../lib/api'
import { exportCSV, fmtDate, fmtRp, fmtTime } from '../lib/store'
import { Button, Modal, PageHead, StatusPill, Td, Th } from '../lib/ui'

const PAGE = 20
const METHODS = ['Semua', 'Cash', 'Bank Transfer', 'QRIS', 'E-Wallet', 'Card']

export default function Transaksi() {
  const [items, setItems] = useState<ApiTrx[] | null>(null)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [qInput, setQInput] = useState('')
  const [q, setQ] = useState('')
  const [method, setMethod] = useState('Semua')
  const [date, setDate] = useState('')
  const [detail, setDetail] = useState<ApiTrx | null>(null)
  const [refundFor, setRefundFor] = useState<ApiTrx | null>(null)
  const [refundQty, setRefundQty] = useState<Record<string, number>>({})
  const [refundReason, setRefundReason] = useState('')
  const [formErr, setFormErr] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const msg = (e: unknown) => (e instanceof ApiError ? e.message : 'Terjadi kasalahan. Coba lagi.')

  const load = useCallback(async (p: number) => {
    try {
      const d = await apiListTransactions({ q, method, date: date || undefined, page: p, limit: PAGE })
      setItems(d.items); setTotal(d.total); setPage(d.page)
    } catch (e) { setErr(msg(e)) }
  }, [q, method, date])

  useEffect(() => {
    const t = setTimeout(() => {
      setQ((prev) => {
        if (prev !== qInput) setPage(1)
        return qInput
      })
    }, 350)
    return () => clearTimeout(t)
  }, [qInput])

  useEffect(() => { load(page) }, [page, q, method, date, load])

  function openRefund(t: ApiTrx) {
    setRefundFor(t)
    setRefundReason('')
    setFormErr('')
  }

  // gabungkan baris produk sama menjadi satu baris dengan total qty
  const refundMerged = useMemo<ApiTrxItem[]>(() => {
    if (!refundFor) return []
    const m = new Map<string, ApiTrxItem>()
    for (const i of refundFor.items) {
      const cur = m.get(i.product_id)
      if (cur) cur.qty += i.qty
      else m.set(i.product_id, { ...i })
    }
    return [...m.values()]
  }, [refundFor])

  useEffect(() => {
    if (refundFor) {
      setRefundQty(Object.fromEntries(refundMerged.map((i) => [i.product_id, i.qty])))
    }
  }, [refundFor, refundMerged])

  async function submitRefund() {
    if (!refundFor) return
    const items = Object.entries(refundQty)
      .filter(([, qty]) => qty > 0)
      .map(([productId, qty]) => ({ productId, qty }))
    if (items.length === 0) return setFormErr('Pilih minimal satu item untuk direfund.')
    if (!refundReason.trim()) return setFormErr('Alasan refund wajib diisi.')
    setBusy(true); setFormErr('')
    try {
      await apiRefundTransaction(refundFor.id, items, refundReason.trim())
      setRefundFor(null)
      await load(page)
    } catch (e) {
      setFormErr(msg(e))
    } finally { setBusy(false) }
  }

  function exportList() {
    if (!items) return
    exportCSV('transaksi.csv', [
      ['id', 'waktu', 'kasir', 'metode', 'subtotal', 'diskon', 'pajak', 'total', 'dibayar', 'kembalian', 'status'],
      ...items.map((t) => [t.id, t.time, t.cashier_name, t.method, String(t.subtotal), String(t.discount), String(t.tax), String(t.total), String(t.paid), String(t.change), t.status]),
    ])
  }

  const pages = Math.max(1, Math.ceil(total / PAGE))

  return (
    <>
      <PageHead
        title="Transaksi"
        sub={total ? `${total} transaksi` : 'Memuat…'}
        right={<Button variant="ghost" onClick={exportList}>Export CSV</Button>}
      />

      {err && <p className="mb-4 rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{err}</p>}

      <div className="mb-4 flex flex-wrap gap-2">
        <input
          value={qInput} onChange={(e) => setQInput(e.target.value)} placeholder="Cari ID atau kasir…"
          className="min-w-52 rounded-md border border-border bg-paper px-3.5 py-2 text-sm focus:border-jet focus:outline-none"
        />
        <input
          type="date" value={date} onChange={(e) => { setDate(e.target.value); setPage(1) }}
          className="rounded-md border border-border bg-paper px-3.5 py-2 text-sm focus:border-jet focus:outline-none"
        />
        {METHODS.map((m) => (
          <button
            key={m}
            onClick={() => { setMethod(m); setPage(1) }}
            className={`rounded-full border px-3 py-1.5 text-xs ${method === m ? 'border-jet bg-jet text-paper' : 'border-dove text-muted hover:border-jet'}`}
          >
            {m}
          </button>
        ))}
      </div>

      <div className="overflow-x-auto rounded-2xl bg-cream p-2">
        {items !== null && items.length === 0 ? (
          <p className="py-14 text-center text-sm text-fog">Tidak ada transaksi ditemukan.</p>
        ) : (
          <table className="w-full border-collapse">
            <thead>
              <tr>
                <Th>ID</Th><Th>Waktu</Th><Th>Kasir</Th><Th>Metode</Th><Th right>Total</Th><Th>Status</Th><Th />
              </tr>
            </thead>
            <tbody>
              {(items ?? []).map((t) => (
                <tr key={t.id}>
                  <Td mono>{t.id}</Td>
                  <Td mono>{fmtDate(t.time)} {fmtTime(t.time)}</Td>
                  <Td>{t.cashier_name}</Td>
                  <Td>{t.method}</Td>
                  <Td right>{fmtRp(t.total)}</Td>
                  <Td><StatusPill status={t.status} /></Td>
                  <Td>
                    <div className="flex justify-end gap-2.5 text-[13px]">
                      <button className="font-medium text-jet hover:underline" onClick={() => setDetail(t)}>Detail</button>
                      {t.status === 'completed' && (
                        <button className="text-muted hover:underline" onClick={() => openRefund(t)}>Refund</button>
                      )}
                    </div>
                  </Td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="mt-4 flex items-center justify-between text-[13px] text-muted">
        <span>{total} transaksi</span>
        <div className="flex gap-2">
          <Button variant="ghost" disabled={page <= 1} onClick={() => setPage(page - 1)}>← Sebelumnya</Button>
          <span className="self-center font-mono text-xs text-fog">{page} / {pages}</span>
          <Button variant="ghost" disabled={page >= pages} onClick={() => setPage(page + 1)}>Berikutnya →</Button>
        </div>
      </div>

      <Modal open={!!detail} title={`Detail ${detail?.id ?? ''}`} onClose={() => setDetail(null)} wide>
        {detail && <TrxDetail t={detail} />}
      </Modal>

      <Modal open={!!refundFor} title={`Refund ${refundFor?.id ?? ''}`} onClose={() => setRefundFor(null)} wide>
        {refundFor && (
          <div className="space-y-4">
            {formErr && <p className="rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{formErr}</p>}
            <p className="text-sm text-muted">Pilih jumlah item yang direfund. Stok akan dikembalikan otomatis.</p>
            <div className="max-h-60 overflow-y-auto rounded-lg border border-dove">
              <table className="w-full border-collapse text-[13px]">
                <thead>
                  <tr className="bg-surface">
                    <Th>Item</Th><Th right>Terjual</Th><Th right>Qty refund</Th>
                  </tr>
                </thead>
                <tbody>
                  {refundMerged.map((i) => (
                    <tr key={i.product_id}>
                      <Td>{i.name}</Td>
                      <Td right>{i.qty}</Td>
                      <Td right>
                        <input
                          type="number" min="0" max={i.qty}
                          value={refundQty[i.product_id] ?? 0}
                          onChange={(e) => setRefundQty((rs) => ({
                            ...rs,
                            [i.product_id]: Math.min(i.qty, Math.max(0, Number(e.target.value))),
                          }))}
                          className="w-16 rounded border border-dove bg-paper px-2 py-1 text-right font-mono tabular-nums focus:border-jet focus:outline-none"
                        />
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <input
              value={refundReason} onChange={(e) => setRefundReason(e.target.value)} placeholder="Alasan refund (wajib)"
              className="w-full rounded-md border border-border bg-paper px-3.5 py-2.5 text-sm focus:border-jet focus:outline-none"
            />
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setRefundFor(null)}>Batal</Button>
              <Button onClick={submitRefund} disabled={busy}>{busy ? 'Memproses…' : 'Proses Refund'}</Button>
            </div>
          </div>
        )}
      </Modal>
    </>
  )
}

function TrxDetail({ t }: { t: ApiTrx }) {
  return (
    <div className="space-y-3 text-sm">
      <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 rounded-lg bg-surface p-4 font-mono text-[13px]">
        <span className="text-fog">Waktu</span><span>{fmtDate(t.time)} {fmtTime(t.time)}</span>
        <span className="text-fog">Kasir</span><span>{t.cashier_name}</span>
        <span className="text-fog">Metode</span><span>{t.method}</span>
        <span className="text-fog">Status</span><span><StatusPill status={t.status} /></span>
      </div>
      <div className="overflow-x-auto rounded-lg border border-dove">
        <table className="w-full border-collapse text-[13px]">
          <thead>
            <tr className="bg-surface">
              <Th>Item</Th><Th right>Harga</Th><Th right>Qty</Th><Th right>Subtotal</Th>
            </tr>
          </thead>
          <tbody>
            {t.items.map((i) => (
              <tr key={i.product_id}>
                <Td>{i.name}</Td>
                <Td right>{fmtRp(i.price)}</Td>
                <Td right>{i.qty}</Td>
                <Td right>{fmtRp(i.price * i.qty)}</Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="space-y-1 font-mono text-[13px]">
        <div className="flex justify-between"><span className="text-fog">Subtotal</span><span>{fmtRp(t.subtotal)}</span></div>
        {t.discount > 0 && <div className="flex justify-between"><span className="text-fog">Diskon</span><span>-{fmtRp(t.discount)}</span></div>}
        {t.tax > 0 && <div className="flex justify-between"><span className="text-fog">Pajak</span><span>{fmtRp(t.tax)}</span></div>}
        <div className="flex justify-between border-t border-dove pt-1.5 font-medium"><span>Total</span><span>{fmtRp(t.total)}</span></div>
        <div className="flex justify-between"><span className="text-fog">Dibayar</span><span>{fmtRp(t.paid)}</span></div>
        <div className="flex justify-between"><span className="text-fog">Kembalian</span><span>{fmtRp(t.change)}</span></div>
      </div>
    </div>
  )
}
