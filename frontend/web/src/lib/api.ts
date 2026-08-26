const BASE: string = import.meta.env.VITE_API_URL ?? '/api/v1'

const ACCESS_KEY = 'op_access'
const REFRESH_KEY = 'op_refresh'

export class ApiError extends Error {
  status: number
  code?: string

  constructor(status: number, message: string, code?: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

function saveTokens(access: string, refresh: string) {
  localStorage.setItem(ACCESS_KEY, access)
  localStorage.setItem(REFRESH_KEY, refresh)
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

interface RequestOptions {
  method?: string
  body?: unknown
  auth?: boolean
}

async function rawFetch(path: string, opts: RequestOptions, accessToken?: string | null): Promise<Response> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (accessToken) headers['Authorization'] = 'Bearer ' + accessToken
  const res = await fetch(BASE + path, {
    method: opts.method ?? (opts.body ? 'POST' : 'GET'),
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
  return res
}

async function parseError(res: Response): Promise<ApiError> {
  let msg = 'Terjadi kesalahan. Coba lagi.'
  let code: string | undefined
  try {
    const data = await res.json()
    if (data?.error) {
      msg = String(data.error)
      // kode spesial dari backend dikirim sebagai pesan
      if (msg === 'passcode_required') code = 'passcode_required'
    }
  } catch {
    /* abaikan */
  }
  return new ApiError(res.status, msg, code)
}

let refreshing: Promise<string | null> | null = null

/** Tukar refresh token dengan pasangan baru. Null bila gagal. */
async function doRefresh(): Promise<string | null> {
  const rt = getRefreshToken()
  if (!rt) return null
  const res = await fetch(BASE + '/auth/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: rt }),
  })
  if (!res.ok) return null
  const data = await res.json()
  if (!data?.access_token || !data?.refresh_token) return null
  saveTokens(data.access_token, data.refresh_token)
  return data.access_token as string
}

/**
 * Fetch ke API dengan penanganan token.
 * - `auth: true` → sertakan access token; bila 401 coba refresh sekali lalu ulang request.
 */
export async function apiFetch<T = any>(path: string, opts: RequestOptions = {}): Promise<T> {
  const useAuth = opts.auth ?? false

  let res = await rawFetch(path, opts, useAuth ? getAccessToken() : null)

  if (useAuth && res.status === 401 && getRefreshToken()) {
    if (!refreshing) refreshing = doRefresh().finally(() => { refreshing = null })
    const fresh = await refreshing
    if (fresh) res = await rawFetch(path, opts, fresh)
  }

  if (!res.ok) throw await parseError(res)

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

// ── endpoint auth ────────────────────────────────────────────────────

export interface AuthUser {
  id: string
  email: string
  name: string
  role: 'admin' | 'cashier'
  store_id: string
  store_name: string
}

interface AuthResponse {
  access_token: string
  refresh_token: string
  user: AuthUser
}

export async function apiLogin(email: string, password: string, passcode?: string): Promise<AuthResponse> {
  const data = await apiFetch<AuthResponse>('/auth/login', {
    body: passcode ? { email, password, passcode } : { email, password },
  })
  saveTokens(data.access_token, data.refresh_token)
  return data
}

export async function apiRegister(name: string, email: string, password: string, storeName: string): Promise<AuthResponse> {
  const data = await apiFetch<AuthResponse>('/auth/register', {
    body: { name, email, password, storeName },
  })
  saveTokens(data.access_token, data.refresh_token)
  return data
}

export async function apiLogout(): Promise<void> {
  const rt = getRefreshToken()
  try {
    if (rt) await apiFetch('/auth/logout', { body: { refresh_token: rt } })
  } catch {
    /* best-effort */
  } finally {
    clearTokens()
  }
}

export async function apiMe(): Promise<AuthUser> {
  const data = await apiFetch<{ user: AuthUser }>('/auth/me', { auth: true })
  return data.user
}

// ── endpoint users (admin) ───────────────────────────────────────────

export interface BackendUser {
  id: string
  email: string
  name: string
  role: 'admin' | 'cashier'
  active: boolean
  store_id: string
  store_name?: string
  created_at?: string
}

export function apiListUsers(): Promise<BackendUser[]> {
  return apiFetch<{ users: BackendUser[] }>('/users', { auth: true }).then((d) => d.users)
}

export function apiCreateUser(name: string, email: string, password: string): Promise<BackendUser> {
  return apiFetch<{ user: BackendUser }>('/users', { auth: true, method: 'POST', body: { name, email, password } }).then((d) => d.user)
}

export function apiSetUserActive(id: string, active: boolean): Promise<void> {
  return apiFetch(`/users/${id}/active`, { auth: true, method: 'PATCH', body: { active } })
}

// ── endpoint katalog: kategori & produk ──────────────────────────────

export interface ApiCategory {
  id: string
  name: string
  active: boolean
  created_at?: string
}

export interface ApiProduct {
  id: string
  category_id: string | null
  category_name: string | null
  name: string
  sku: string
  barcode: string
  buy_price: number
  sell_price: number
  stock: number
  unit: string
  active: boolean
  created_at?: string
}

export interface ProductPage {
  items: ApiProduct[]
  total: number
  page: number
  limit: number
}

export function apiListCategories(): Promise<ApiCategory[]> {
  return apiFetch<{ categories: ApiCategory[] }>('/categories', { auth: true }).then((d) => d.categories)
}

export function apiCreateCategory(name: string): Promise<ApiCategory> {
  return apiFetch<{ category: ApiCategory }>('/categories', { auth: true, method: 'POST', body: { name } }).then((d) => d.category)
}

/** Menghapus kategori; bila masih dipakai produk server menonaktifkannya (soft-delete). */
export function apiDeleteCategory(id: string): Promise<{ soft_deleted: boolean }> {
  return apiFetch(`/categories/${id}`, { auth: true, method: 'DELETE' })
}

export function apiListProducts(opts: { q?: string; categoryId?: string; active?: boolean; page?: number; limit?: number } = {}): Promise<ProductPage> {
  const sp = new URLSearchParams()
  if (opts.q?.trim()) sp.set('q', opts.q.trim())
  if (opts.categoryId) sp.set('categoryId', opts.categoryId)
  if (opts.active !== undefined) sp.set('active', String(opts.active))
  sp.set('page', String(opts.page ?? 1))
  sp.set('limit', String(opts.limit ?? 20))
  return apiFetch<ProductPage>(`/products?${sp.toString()}`, { auth: true })
}

export function apiCreateProduct(body: {
  name: string; sku: string; barcode?: string; categoryId?: string | null;
  buyPrice?: number; sellPrice: number; stock?: number; unit?: string;
}): Promise<ApiProduct> {
  return apiFetch<ApiProduct>('/products', { auth: true, method: 'POST', body })
}

export function apiUpdateProduct(id: string, body: {
  name: string; sku: string; barcode?: string; categoryId?: string | null;
  buyPrice?: number; sellPrice: number; unit?: string;
}): Promise<ApiProduct> {
  return apiFetch<ApiProduct>(`/products/${id}`, { auth: true, method: 'PUT', body })
}

export function apiSetProductActive(id: string, active: boolean): Promise<void> {
  return apiFetch(`/products/${id}/active`, { auth: true, method: 'PATCH', body: { active } })
}

// ── endpoint stok: penyesuaian & riwayat pergerakan ──────────────────

export type MovementType = 'sale' | 'refund' | 'adjust' | 'initial'

export interface ApiMovement {
  id: string
  product_id: string
  product_name: string | null
  type: MovementType
  qty: number // negatif = keluar, positif = masuk
  reason: string
  actor: string
  created_at?: string
}

export interface MovementPage {
  items: ApiMovement[]
  total: number
  page: number
  limit: number
}

export function apiListMovements(opts: { type?: MovementType; productId?: string; page?: number; limit?: number } = {}): Promise<MovementPage> {
  const sp = new URLSearchParams()
  if (opts.type) sp.set('type', opts.type)
  if (opts.productId) sp.set('productId', opts.productId)
  sp.set('page', String(opts.page ?? 1))
  sp.set('limit', String(opts.limit ?? 25))
  return apiFetch<MovementPage>(`/movements?${sp.toString()}`, { auth: true })
}

/** Penyesuaian stok admin; gagal bila hasil akhir negatif. */
export function apiAdjustStock(productId: string, direction: 'plus' | 'minus', qty: number, reason: string): Promise<ApiProduct> {
  return apiFetch<{ product: ApiProduct }>('/stock/adjustments', {
    auth: true, method: 'POST',
    body: { productId, direction, qty, reason },
  }).then((d) => d.product)
}

// ── endpoint transaksi & refund ──────────────────────────────────────

export interface ApiTrxItem {
  product_id: string
  name: string
  buy_price: number
  price: number
  qty: number
}

export type TrxStatus = 'pending' | 'completed' | 'cancelled' | 'refunded'

export interface ApiTrx {
  id: string
  seq: number
  cashier_name: string
  items: ApiTrxItem[]
  subtotal: number
  discount: number
  tax: number
  total: number
  method: string
  paid: number
  change: number
  status: TrxStatus
  customer?: string
  time: string
}

export function apiCheckout(body: {
  items: { productId: string; qty: number }[]
  discount?: number
  method: string
  paid?: number
  customer?: string
}): Promise<ApiTrx> {
  return apiFetch<ApiTrx>('/transactions', { auth: true, method: 'POST', body })
}

export function apiListTransactions(opts: { q?: string; method?: string; date?: string; page?: number; limit?: number } = {}): Promise<{ items: ApiTrx[]; total: number; page: number; limit: number }> {
  const sp = new URLSearchParams()
  if (opts.q?.trim()) sp.set('q', opts.q.trim())
  if (opts.method && opts.method !== 'Semua') sp.set('method', opts.method)
  if (opts.date) sp.set('date', opts.date)
  sp.set('page', String(opts.page ?? 1))
  sp.set('limit', String(opts.limit ?? 20))
  return apiFetch(`/transactions?${sp.toString()}`, { auth: true })
}

/** Refund parsial/penuh (admin); kembalikan transaksi dengan status terbaru. */
export function apiRefundTransaction(trxId: string, items: { productId: string; qty: number }[], reason: string): Promise<ApiTrx> {
  return apiFetch<ApiTrx>(`/transactions/${trxId}/refund`, { auth: true, method: 'POST', body: { items, reason } })
}

// ── endpoint settings toko ───────────────────────────────────────────

export interface ApiSettings {
  storeName: string
  address: string
  phone: string
  taxEnabled: boolean
  taxPct: number
  receiptHeader: string
  receiptFooter: string
  paper: string
  timezone: string
}

export function apiGetSettings(): Promise<ApiSettings> {
  return apiFetch<ApiSettings>('/settings', { auth: true })
}

export function apiUpdateSettings(s: ApiSettings): Promise<ApiSettings> {
  return apiFetch<ApiSettings>('/settings', { auth: true, method: 'PUT', body: s })
}

/** Passcode 5 angka; string kosong = menghapus passcode. */
export function apiSetPasscode(userId: string, passcode: string): Promise<void> {
  return apiFetch(`/users/${userId}/passcode`, { auth: true, method: 'PUT', body: { passcode } })
}

// ── endpoint dashboard & laporan ─────────────────────────────────────

export interface DayPoint { date: string; omzet: number }
export interface MethodPoint { method: string; total: number }
export interface TopProduct { product_id: string; name: string; qty: number; revenue: number }
export interface TrxBrief { id: string; cashier_name: string; total: number; status: TrxStatus; time: string }

interface DashboardToday { omzet: number; trx_count: number; items_sold: number; low_stock?: number }

export interface DashboardAdminDto {
  role: 'admin'
  today: DashboardToday
  sales7: DayPoint[]
  methods: MethodPoint[]
  top_products: TopProduct[]
  recent: TrxBrief[]
}

export interface DashboardCashierDto {
  role: 'cashier'
  today: DashboardToday
  recent: TrxBrief[]
}

export type DashboardDto = DashboardAdminDto | DashboardCashierDto

export function apiGetDashboard(): Promise<DashboardDto> {
  return apiFetch<DashboardDto>('/dashboard', { auth: true })
}

export interface ReportSummary { omzet: number; trx_count: number; items_sold: number; gross_profit: number }
export interface ProductReportRow { product_id: string; name: string; sku: string; qty: number; revenue: number; profit: number }
export interface TrxProfitRow { date: string; id: string; cashier: string; method: string; total: number; hpp: number; profit: number; status: TrxStatus }
export interface StockRow { name: string; sku: string; stock: number; buy_price: number; sell_price: number; stock_value: number }
export interface StatusCount { status: TrxStatus; count: number }

export interface ReportBundle {
  period: string
  summary: ReportSummary
  by_method: MethodPoint[]
  by_status: StatusCount[]
  products: ProductReportRow[]
  transactions: TrxProfitRow[]
  stock: StockRow[]
}

export type ReportPeriod = 'today' | 'yesterday' | 'week' | 'month' | 'all'

export function apiGetReport(period: ReportPeriod): Promise<ReportBundle> {
  return apiFetch<ReportBundle>(`/reports?period=${period}`, { auth: true })
}
