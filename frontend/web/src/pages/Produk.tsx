import { useCallback, useEffect, useRef, useState } from 'react'
import {
  apiCreateCategory, apiCreateProduct, apiDeleteCategory,
  apiListCategories, apiListProducts, apiSetProductActive, apiUpdateProduct,
  type ApiCategory, type ApiProduct, ApiError,
} from '../lib/api'
import { exportCSV, fmtRp } from '../lib/store'
import { Button, Empty, Input, Modal, PageHead, Pill, Td, Th } from '../lib/ui'

interface Draft {
  id?: string
  name: string
  sku: string
  barcode: string
  categoryId: string
  buyPrice: string
  sellPrice: string
  stock: string
  unit: string
}

const emptyDraft: Draft = { name: '', sku: '', barcode: '', categoryId: '', buyPrice: '', sellPrice: '', stock: '', unit: 'pcs' }
const PAGE_SIZE = 10

export default function Produk() {
  const [cats, setCats] = useState<ApiCategory[]>([])
  const [items, setItems] = useState<ApiProduct[] | null>(null)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [qInput, setQInput] = useState('')
  const [q, setQ] = useState('')
  const [err, setErr] = useState('')

  const [editing, setEditing] = useState<Draft | null>(null)
  const [formErr, setFormErr] = useState('')
  const [catName, setCatName] = useState('')
  const [importRows, setImportRows] = useState<{ ok: boolean; row: string[]; msg: string }[] | null>(null)
  const [busy, setBusy] = useState(false)

  const loadProducts = useCallback(async (p: number, query: string) => {
    try {
      const d = await apiListProducts({ q: query, page: p, limit: PAGE_SIZE })
      setItems(d.items); setTotal(d.total); setPage(d.page)
    } catch (e) { setErr(msg(e)) }
  }, [])

  const loadCats = useCallback(async () => {
    try { setCats(await apiListCategories()) } catch (e) { setErr(msg(e)) }
  }, [])

  useEffect(() => { loadCats() }, [loadCats])

  // debounce pencarian: qInput → q (reset ke halaman 1)
  useEffect(() => {
    const t = setTimeout(() => {
      setQ((prev) => {
        if (prev !== qInput) setPage(1)
        return qInput
      })
    }, 350)
    return () => clearTimeout(t)
  }, [qInput])

  useEffect(() => { loadProducts(page, q) }, [page, q, loadProducts])

  async function refreshAll() {
    await Promise.all([loadCats(), loadProducts(page, q)])
  }

  function msg(e: unknown): string {
    return e instanceof ApiError ? e.message : 'Terjadi kesalahan. Coba lagi.'
  }

  async function save(d: Draft) {
    const sell = Number(d.sellPrice)
    if (!d.name.trim() || !d.sku.trim() || !Number.isFinite(sell) || d.sellPrice === '') {
      setFormErr('Nama, SKU, dan harga jual wajib diisi.')
      return
    }
    const body = {
      name: d.name.trim(),
      sku: d.sku.trim(),
      barcode: d.barcode.trim(),
      categoryId: d.categoryId || null,
      buyPrice: Number(d.buyPrice) || 0,
      sellPrice: sell,
      unit: d.unit.trim() || 'pcs',
    }
    setBusy(true); setFormErr('')
    try {
      if (d.id) await apiUpdateProduct(d.id, body)
      else await apiCreateProduct({ ...body, stock: Number(d.stock) || 0 })
      setEditing(null)
      await refreshAll()
    } catch (e) {
      setFormErr(msg(e))
    } finally { setBusy(false) }
  }

  async function toggleActive(p: ApiProduct) {
    setErr('')
    try { await apiSetProductActive(p.id, !p.active); await loadProducts(page, q) }
    catch (e) { setErr(msg(e)) }
  }

  async function addCat() {
    const n = catName.trim()
    if (!n) return
    setErr('')
    try { await apiCreateCategory(n); setCatName(''); await loadCats() }
    catch (e) { setErr(msg(e)) }
  }

  async function deleteCat(c: ApiCategory) {
    setErr('')
    try {
      const r = await apiDeleteCategory(c.id)
      if (r.soft_deleted) window.alert('Kategori masih dipakai produk — dinonaktifkan saja agar histori tetap utuh.')
      await refreshAll()
    } catch (e) { setErr(msg(e)) }
  }

  async function fetchAllForExport(): Promise<ApiProduct[]> {
    const out: ApiProduct[] = []
    let p = 1
    for (;;) {
      const d = await apiListProducts({ page: p, limit: 200 })
      out.push(...d.items)
      if (out.length >= d.total || d.items.length === 0) break
      p++
    }
    return out
  }

  async function exportList() {
    setErr('')
    try {
      const all = await fetchAllForExport()
      exportCSV('produk.csv', [
        ['nama', 'sku', 'barcode', 'kategori', 'harga_beli', 'harga_jual', 'stok', 'unit', 'aktif'],
        ...all.map((p) => [p.name, p.sku, p.barcode, p.category_name ?? '', String(p.buy_price), String(p.sell_price), String(p.stock), p.unit, p.active ? '1' : '0']),
      ])
    } catch (e) { setErr(msg(e)) }
  }

  const fileRef = useRef<HTMLInputElement>(null)

  function onImportFile(f: File) {
    const reader = new FileReader()
    reader.onload = () => {
      const lines = String(reader.result).split(/\r?\n/).filter((l) => l.trim())
      const rows = lines.slice(1).map((l) => l.split(',').map((c) => c.trim().replace(/^"|"$/g, '')))
      const seenSku = new Set<string>()
      const parsed = rows.map((row, i) => {
        const [name, sku, , sell] = row
        if (!name || !sku || !sell) return { ok: false, row, msg: `Baris ${i + 2}: nama/SKU/harga jual wajib diisi` }
        const key = sku.toLowerCase()
        if (seenSku.has(key)) return { ok: false, row, msg: `Baris ${i + 2}: SKU duplikat di dalam berkas` }
        seenSku.add(key)
        return { ok: true, row, msg: 'siap diimpor' }
      })
      setImportRows(parsed)
    }
    reader.readAsText(f)
  }

  async function commitImport() {
    if (!importRows) return
    setBusy(true)
    let okCount = 0
    const results = [...importRows]
    for (let i = 0; i < results.length; i++) {
      if (!results[i].ok) continue
      const [name, sku, buy, sell, stock, cat, barcode] = results[i].row
      const categoryId = cats.find((c) => c.active && c.name.toLowerCase() === (cat ?? '').toLowerCase())?.id ?? null
      try {
        await apiCreateProduct({
          name, sku, barcode: barcode ?? '', categoryId,
          buyPrice: Number(buy) || 0, sellPrice: Number(sell), stock: Number(stock) || 0, unit: 'pcs',
        })
        results[i] = { ...results[i], ok: false, msg: 'terimpor ✓' }
        okCount++
      } catch (e) {
        results[i] = { ...results[i], ok: false, msg: msg(e) }
      }
    }
    setImportRows(results)
    setBusy(false)
    await refreshAll()
    window.alert(`${okCount} produk terimpor.`)
  }

  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const activeCats = cats.filter((c) => c.active)
  const inactiveCats = cats.filter((c) => !c.active)

  return (
    <>
      <PageHead
        title="Produk"
        sub={items ? `${total} produk · ${activeCats.length} kategori aktif` : 'Memuat…'}
        right={
          <div className="flex gap-2">
            <Button variant="ghost" onClick={exportList}>Export CSV</Button>
            <Button variant="ghost" onClick={() => fileRef.current?.click()}>Import CSV</Button>
            <Button onClick={() => { setEditing({ ...emptyDraft }); setFormErr('') }}>+ Tambah Produk</Button>
            <input ref={fileRef} type="file" accept=".csv" hidden onChange={(e) => { const f = e.target.files?.[0]; if (f) onImportFile(f); e.target.value = '' }} />
          </div>
        }
      />

      {err && <p className="mb-4 rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{err}</p>}

      <div className="mb-5 grid gap-4 lg:grid-cols-[1fr_260px]">
        <div>
          <input
            value={qInput} onChange={(e) => setQInput(e.target.value)} placeholder="Cari nama atau SKU…"
            className="mb-4 w-full rounded-md border border-border bg-paper px-3.5 py-2.5 text-sm focus:border-jet focus:outline-none"
          />
          <div className="overflow-x-auto rounded-2xl bg-cream p-2">
            {items !== null && items.length === 0 ? (
              <Empty title="Belum ada produk" sub="Tambah produk pertama untuk mulai berjualan." action={<Button onClick={() => setEditing({ ...emptyDraft })}>+ Tambah Produk</Button>} />
            ) : (
              <table className="w-full border-collapse">
                <thead>
                  <tr>
                    <Th>Nama</Th><Th>SKU</Th><Th>Kategori</Th><Th right>Beli</Th><Th right>Jual</Th><Th right>Stok</Th><Th>Status</Th><Th />
                  </tr>
                </thead>
                <tbody>
                  {(items ?? []).map((p) => (
                    <tr key={p.id}>
                      <Td mono={false}><span className="font-medium text-fg">{p.name}</span></Td>
                      <Td mono>{p.sku}</Td>
                      <Td>{p.category_name ?? '—'}</Td>
                      <Td right>{fmtRp(p.buy_price)}</Td>
                      <Td right>{fmtRp(p.sell_price)}</Td>
                      <Td right><span className={p.stock <= 5 ? 'font-medium text-ember' : ''}>{p.stock}</span></Td>
                      <Td><Pill tone={p.active ? 'ok' : 'muted'}>{p.active ? 'Aktif' : 'Nonaktif'}</Pill></Td>
                      <Td>
                        <div className="flex justify-end gap-2 text-[13px]">
                          <button className="font-medium text-jet hover:underline" onClick={() => setEditing({ id: p.id, name: p.name, sku: p.sku, barcode: p.barcode, categoryId: p.category_id ?? '', buyPrice: String(p.buy_price), sellPrice: String(p.sell_price), stock: String(p.stock), unit: p.unit })}>Ubah</button>
                          <button className="text-muted hover:underline" onClick={() => toggleActive(p)}>{p.active ? 'Nonaktifkan' : 'Aktifkan'}</button>
                        </div>
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
          {total > PAGE_SIZE && (
            <div className="mt-3 flex items-center justify-between text-[13px] text-muted">
              <span>{total} produk</span>
              <div className="flex gap-2">
                <Button variant="ghost" disabled={page <= 1} onClick={() => setPage(page - 1)}>← Sebelumnya</Button>
                <span className="self-center font-mono text-xs text-fog">{page} / {pages}</span>
                <Button variant="ghost" disabled={page >= pages} onClick={() => setPage(page + 1)}>Berikutnya →</Button>
              </div>
            </div>
          )}
        </div>

        <aside className="rounded-2xl bg-cream p-5 self-start">
          <h2 className="font-mono text-xs uppercase tracking-wider text-fog">Kategori</h2>
          <div className="mt-3 space-y-1.5">
            {activeCats.map((c) => (
              <div key={c.id} className="flex items-center justify-between rounded-lg border border-dove bg-paper px-3 py-2 text-sm">
                <span>{c.name}</span>
                <button onClick={() => deleteCat(c)} className="text-xs text-fog hover:text-ember">hapus</button>
              </div>
            ))}
            {inactiveCats.length > 0 && (
              <p className="pt-1 text-xs text-fog">{inactiveCats.length} kategori dinonaktifkan (historis)</p>
            )}
          </div>
          <div className="mt-4 flex gap-2">
            <input
              value={catName} onChange={(e) => setCatName(e.target.value)} placeholder="Nama kategori baru"
              className="min-w-0 flex-1 rounded-md border border-border bg-paper px-3 py-2 text-sm focus:border-jet focus:outline-none"
              onKeyDown={(e) => { if (e.key === 'Enter') addCat() }}
            />
            <Button onClick={addCat}>Tambah</Button>
          </div>
        </aside>
      </div>

      <Modal open={!!editing} title={editing?.id ? 'Ubah Produk' : 'Tambah Produk'} onClose={() => setEditing(null)} wide>
        {editing && (
          <FormProduk draft={editing} cats={activeCats} onSave={save} onCancel={() => setEditing(null)} formErr={formErr} busy={busy} />
        )}
      </Modal>

      <Modal open={!!importRows} title="Preview Import" onClose={() => setImportRows(null)} wide>
        {importRows && (
          <div>
            <div className="max-h-72 overflow-y-auto rounded-lg border border-dove">
              <table className="w-full border-collapse text-[13px]">
                <thead>
                  <tr className="bg-surface">
                    <Th>Baris</Th><Th>Nama</Th><Th>SKU</Th><Th>Hasil</Th>
                  </tr>
                </thead>
                <tbody>
                  {importRows.map((r, i) => (
                    <tr key={i}>
                      <Td mono>{i + 2}</Td>
                      <Td>{r.row[0]}</Td>
                      <Td mono>{r.row[1]}</Td>
                      <Td><span className={!r.ok && r.msg.startsWith('terimpor') ? 'text-sprout' : r.ok ? 'text-sprout' : r.msg.includes('✓') ? 'text-sprout' : 'text-ember'}>{r.msg}</span></Td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="mt-4 flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setImportRows(null)}>Tutup</Button>
              <Button onClick={commitImport} disabled={busy || importRows.every((r) => !r.ok)}>
                Impor {importRows.filter((r) => r.ok).length} Produk
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </>
  )
}

function FormProduk({ draft, cats, onSave, onCancel, formErr, busy }: {
  draft: Draft
  cats: ApiCategory[]
  onSave: (d: Draft) => void
  onCancel: () => void
  formErr: string
  busy: boolean
}) {
  const [d, setD] = useState(draft)
  const set = (k: keyof Draft) => (v: string) => setD({ ...d, [k]: v })
  return (
    <form
      className="grid gap-4 sm:grid-cols-2"
      onSubmit={(e) => { e.preventDefault(); onSave(d) }}
    >
      {formErr && <p className="rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember sm:col-span-2">{formErr}</p>}
      <div className="sm:col-span-2">
        <Input label="Nama produk" value={d.name} onChange={set('name')} required />
      </div>
      <Input label="SKU (unik)" value={d.sku} onChange={set('sku')} required />
      <Input label="Barcode (opsional)" value={d.barcode} onChange={set('barcode')} />
      <label className="flex flex-col gap-1.5 text-[13px] font-medium text-steel">
        Kategori
        <select value={d.categoryId} onChange={(e) => set('categoryId')(e.target.value)} className="rounded-md border border-border bg-paper px-3.5 py-2.5 text-[15px] focus:border-jet focus:outline-none">
          <option value="">— tanpa kategori —</option>
          {cats.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </label>
      <Input label="Satuan" value={d.unit} onChange={set('unit')} />
      <Input label="Harga beli (Rp)" type="number" value={d.buyPrice} onChange={set('buyPrice')} />
      <Input label="Harga jual (Rp)" type="number" value={d.sellPrice} onChange={set('sellPrice')} required />
      {!d.id && <Input label="Stok awal" type="number" value={d.stock} onChange={set('stock')} />}
      {d.id && <p className="self-end text-xs text-fog sm:col-span-1">Stok diubah lewat menu Stok / transaksi.</p>}
      <div className="mt-2 flex justify-end gap-2 sm:col-span-2">
        <Button type="button" variant="ghost" onClick={onCancel}>Batal</Button>
        <Button type="submit" disabled={busy}>{busy ? 'Menyimpan…' : 'Simpan'}</Button>
      </div>
    </form>
  )
}
