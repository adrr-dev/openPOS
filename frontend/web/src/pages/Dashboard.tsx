import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { Banknote, Package, ReceiptText, TriangleAlert, Store } from 'lucide-react'
import {
  apiGetDashboard, apiGetSettings,
  type ApiSettings, type DashboardAdminDto, type DashboardCashierDto,
  type DashboardDto,
} from '../lib/api'
import { fmtRp } from '../lib/store'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart'
import { Area, AreaChart, CartesianGrid, Pie, PieChart, XAxis, YAxis } from 'recharts'
import { Button } from '@/components/ui/button'
import { Empty, EmptyContent, EmptyDescription, EmptyTitle } from '@/components/ui/empty'

const DAYS = ['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab']

const salesConfig = { omzet: { label: 'Omzet', color: 'var(--chart-1)' } } as const
const payConfig = {
  Cash: { label: 'Cash', color: 'var(--chart-1)' },
  'Bank Transfer': { label: 'Bank Transfer', color: 'var(--chart-2)' },
  QRIS: { label: 'QRIS', color: 'var(--chart-3)' },
  'E-Wallet': { label: 'E-Wallet', color: 'var(--chart-4)' },
  Card: { label: 'Card', color: 'var(--chart-5)' },
} as const

function dayLabel(iso: string): string {
  const d = new Date(iso + 'T00:00:00')
  return DAYS[d.getDay()]
}

export default function Dashboard() {
  const [data, setData] = useState<DashboardDto | null>(null)
  const [settings, setSettings] = useState<ApiSettings | null>(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    apiGetDashboard().then(setData).catch((e) => setErr(e.message))
    apiGetSettings().then(setSettings).catch(() => {})
  }, [])

  const admin = data?.role === 'admin' ? (data as DashboardAdminDto) : null
  const cashier = data?.role === 'cashier' ? (data as DashboardCashierDto) : null

  const chart7 = useMemo(
    () => (admin ? admin.sales7.map((d) => ({ day: dayLabel(d.date), omzet: d.omzet })) : []),
    [admin],
  )
  const payData = useMemo(
    () =>
      admin
        ? admin.methods
            .map((m) => ({
              name: m.method,
              total: m.total,
              fill: (payConfig as Record<string, { color: string }>)[m.method]?.color ?? 'var(--chart-1)',
            }))
            .filter((d) => d.total > 0)
        : [],
    [admin],
  )
  const topMax = useMemo(() => Math.max(...(admin?.top_products ?? []).map((p) => p.qty), 1), [admin])
  const storeName = settings?.storeName ?? ''

  if (err) {
    return <p className="rounded-lg bg-sand px-3.5 py-2.5 text-[13px] text-ember">{err}</p>
  }
  if (!data) {
    return <p className="py-10 text-center text-sm text-fog">Memuat dashboard…</p>
  }

  const today = admin ?? cashier!
  const kpis = [
    { label: 'Omzet hari ini', value: fmtRp(today.today.omzet), sub: 'dari semua metode bayar', icon: Banknote },
    { label: 'Transaksi hari ini', value: String(today.today.trx_count), sub: 'selesai · tercatat otomatis', icon: ReceiptText },
    { label: 'Produk terjual', value: String(today.today.items_sold), sub: 'satuan terjual hari ini', icon: Package },
    ...(admin ? [{ label: 'Stok menipis', value: String(admin.today.low_stock), sub: 'perlu di-restock', icon: TriangleAlert }] : []),
  ]

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">Ringkasan toko</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Halo {today.recent[0]?.cashier_name ?? ''} — berikut performa {storeName || 'toko'} hari ini.
          </p>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {kpis.map((k) => (
          <Card key={k.label}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">{k.label}</CardTitle>
              <k.icon className="size-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <p className="font-mono text-2xl font-medium tabular-nums">{k.value}</p>
              <p className="mt-1 font-mono text-xs text-muted-foreground">{k.sub}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {admin && (
        <>
          <div className="grid gap-4 lg:grid-cols-[1.6fr_1fr]">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Penjualan 7 hari terakhir</CardTitle>
                <CardDescription>Omzet per hari</CardDescription>
              </CardHeader>
              <CardContent>
                <ChartContainer config={salesConfig} className="h-56 w-full">
                  <AreaChart data={chart7} margin={{ top: 4, right: 4, bottom: 0, left: 4 }}>
                    <defs>
                      <linearGradient id="fillOmzet" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="var(--color-omzet)" stopOpacity={0.35} />
                        <stop offset="95%" stopColor="var(--color-omzet)" stopOpacity={0.02} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid vertical={false} strokeDasharray="4 4" className="stroke-border" />
                    <XAxis dataKey="day" tickLine={false} axisLine={false} tickMargin={8} className="font-mono text-xs" />
                    <YAxis tickLine={false} axisLine={false} width={44} tickFormatter={(v: number) => shortRp(v)} className="font-mono text-xs" />
                    <ChartTooltip cursor={false} content={<ChartTooltipContent formatter={(v) => fmtRp(Number(v))} />} />
                    <Area dataKey="omzet" type="natural" fill="url(#fillOmzet)" stroke="var(--color-omzet)" strokeWidth={2} />
                  </AreaChart>
                </ChartContainer>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Metode pembayaran</CardTitle>
                <CardDescription>Hari ini</CardDescription>
              </CardHeader>
              <CardContent>
                {payData.length === 0 ? (
                  <p className="py-12 text-center font-mono text-xs text-muted-foreground">Belum ada transaksi hari ini.</p>
                ) : (
                  <div className="space-y-4">
                    <ChartContainer config={payConfig} className="mx-auto h-40 w-full">
                      <PieChart>
                        <ChartTooltip content={<ChartTooltipContent formatter={(v) => fmtRp(Number(v))} />} />
                        <Pie data={payData} dataKey="total" nameKey="name" innerRadius={52} outerRadius={72} paddingAngle={3} strokeWidth={0} />
                      </PieChart>
                    </ChartContainer>
                    <div className="space-y-2">
                      {payData.map((d) => (
                        <div key={d.name} className="flex items-center justify-between gap-3 text-sm">
                          <span className="flex items-center gap-2">
                            <span className="size-2.5 rounded-full" style={{ background: d.fill }} />
                            {d.name}
                          </span>
                          <span className="font-mono text-xs tabular-nums text-muted-foreground">{fmtRp(d.total)}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          <div className="grid gap-4 lg:grid-cols-[1.6fr_1fr]">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Produk terlaris</CardTitle>
                <CardDescription>Hari ini</CardDescription>
              </CardHeader>
              <CardContent>
                {admin.top_products.length === 0 ? (
                  <p className="py-12 text-center font-mono text-xs text-muted-foreground">Belum ada penjualan hari ini.</p>
                ) : (
                  <div className="space-y-4">
                    {admin.top_products.map((p) => (
                      <div key={p.product_id} className="space-y-1.5">
                        <div className="flex items-baseline justify-between gap-3 text-sm">
                          <span className="truncate font-medium">{p.name}</span>
                          <span className="font-mono text-xs tabular-nums text-muted-foreground">{p.qty} pcs</span>
                        </div>
                        <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                          <div className="h-full rounded-full bg-foreground" style={{ width: `${Math.round((p.qty / topMax) * 100)}%` }} />
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            <RecentCard recent={admin.recent} />
          </div>

          {admin.today.trx_count === 0 && (
            <Empty>
              <EmptyContent>
                <EmptyTitle>Belum ada transaksi hari ini</EmptyTitle>
                <EmptyDescription>Buka menu POS Kasir untuk memulai transaksi pertama.</EmptyDescription>
                <Button render={<Link to="/app/pos" />}>Buka POS Kasir</Button>
              </EmptyContent>
            </Empty>
          )}
        </>
      )}

      {cashier && (
        <>
          <RecentCard recent={cashier.recent} title="Transaksi shift Anda" />
          <Card className="border-dashed">
            <CardContent className="flex flex-wrap items-center justify-between gap-4 p-5">
              <div className="flex items-center gap-3">
                <div className="grid size-10 place-items-center rounded-lg bg-muted">
                  <Store className="size-5" />
                </div>
                <div>
                  <p className="text-sm font-medium">Ringkasan shift Anda</p>
                  <p className="font-mono text-xs text-muted-foreground">
                    {cashier.today.trx_count} transaksi · {fmtRp(cashier.today.omzet)} — hanya transaksi yang Anda buat.
                  </p>
                </div>
              </div>
              <Button render={<Link to="/app/pos" />}>Buka POS</Button>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}

function RecentCard({ recent, title = 'Transaksi terbaru' }: { recent: { id: string; time: string; cashier_name: string; total: number; status: string }[]; title?: string }) {
  const badge = (s: string) => {
    const map: Record<string, { label: string; variant: 'default' | 'secondary' | 'outline' }> = {
      completed: { label: 'Selesai', variant: 'default' },
      pending: { label: 'Proses', variant: 'outline' },
      cancelled: { label: 'Dibatalkan', variant: 'secondary' },
      refunded: { label: 'Refund', variant: 'secondary' },
    }
    const b = map[s] ?? { label: s, variant: 'outline' as const }
    return <Badge variant={b.variant}>{b.label}</Badge>
  }
  const t = (iso: string) => new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle className="text-base">{title}</CardTitle>
          <CardDescription>{recent.length} terakhir</CardDescription>
        </div>
        <Button variant="outline" size="sm" render={<Link to="/app/transaksi" />}>Lihat semua</Button>
      </CardHeader>
      <CardContent>
        {recent.length === 0 ? (
          <p className="py-12 text-center font-mono text-xs text-muted-foreground">Belum ada transaksi.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Waktu</TableHead>
                <TableHead>Kasir</TableHead>
                <TableHead className="text-right">Total</TableHead>
                <TableHead className="text-right">Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {recent.map((r) => (
                <TableRow key={r.id}>
                  <TableCell className="font-mono text-xs">{r.id}</TableCell>
                  <TableCell className="font-mono text-xs">{t(r.time)}</TableCell>
                  <TableCell>{r.cashier_name}</TableCell>
                  <TableCell className="text-right font-mono text-xs tabular-nums">{fmtRp(r.total)}</TableCell>
                  <TableCell className="text-right">{badge(r.status)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

function shortRp(n: number): string {
  if (n >= 1000000) return (n / 1000000).toLocaleString('id-ID', { maximumFractionDigits: 1 }) + 'jt'
  if (n >= 1000) return Math.round(n / 1000).toLocaleString('id-ID') + 'rb'
  return String(n)
}
