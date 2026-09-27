import type { BusinessRecordInput } from '@/api/admin/business'

export const businessKindLabels: Record<string, string> = {
  receipt: '线下收款', payment_orders: '线上支付 / 退款', purchase: '上游采购', expense: '经营费用', expense_stop: '预付费用终止',
  opening_pool: '期初采购', opening_unknown: '期初余额', user_subscriptions: '订阅购买期',
  wallet: '余额来源', signup_wallet: '注册赠送', usage: '调用成本', annotation: '来源修订', cost_repair_batch: '历史成本批量补算',
  adjustment: '账务调整', reconciliation: '账单差异', reversal: '冲销', supplier_refund: '采购退款', supplier_loss: '采购失效',
  wallet_revenue: '余额消费收入', subscription_revenue: '订阅服务收入', subscription_close_revenue: '订阅终止结转',
  ticket_revenue: '抽奖券消费收入', refund_revenue: '退款收入冲回', usage_cost: '上游消耗成本', fixed_cost: '服务期成本',
  operating_cost: '经营费用', cash_in: '现金收款', cash_out: '现金付款', cash_refund: '现金退款',
  gift_grant: '赠送发放', gift_use: '赠送使用', gift_cost: '赠送兑现成本（已含在总成本）', discount: '成交优惠',
  revenue_gap: '收入待核对', cost_gap: '成本待核对', cash_gap: '现金待核对', wallet_adjustment: '余额变动待核对', history_gap: '历史待补录',
}
export const businessQuality = (value: string) => ({ confirmed: '已核实', contract: '合同计价', estimated: '暂估', allocated: '分摊', direct: '直接归属', unknown: '待核对', pending: '计算中' }[value] || value)
export function cny(value: string | null | undefined): string {
  if (value == null) return '待核对'
  // Only presentation uses Number. Accounting and API amounts remain decimals.
  const n = Number(value)
  return Number.isFinite(n) ? new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(n) : '待核对'
}
export function ledgerError(e: unknown): string {
  const err = e as { response?: { data?: { message?: string } }; message?: string }
  return err?.response?.data?.message || err?.message || '操作失败，请重试'
}
export function localDateTime(): string {
  const now = new Date()
  return new Date(now.getTime() - now.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
export function parseLedgerCSV(text: string): BusinessRecordInput[] {
  const rows: string[][] = []; let row: string[] = []; let cell = ''; let quoted = false
  text = text.replace(/^\uFEFF/, '')
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (ch === '"') { if (quoted && text[i + 1] === '"') { cell += '"'; i++ } else quoted = !quoted }
    else if (ch === ',' && !quoted) { row.push(cell); cell = '' }
    else if ((ch === '\n' || ch === '\r') && !quoted) {
      if (ch === '\r' && text[i + 1] === '\n') i++
      row.push(cell); if (row.some(Boolean)) rows.push(row); row = []; cell = ''
    } else cell += ch
  }
  if (quoted) throw new Error('CSV 引号未闭合')
  row.push(cell); if (row.some(Boolean)) rows.push(row)
  const headers = rows.shift()?.map(v => v.trim()) || []
  for (const field of ['idempotency_key', 'type', 'occurred_at', 'amount_cny', 'notes']) if (!headers.includes(field)) throw new Error(`缺少列 ${field}`)
  if (!rows.length || rows.length > 500) throw new Error('每次导入 1–500 条记录')
  const keys = new Set<string>()
  return rows.map((values, i) => {
    if (values.length !== headers.length) throw new Error(`第 ${i + 2} 行列数不匹配`)
    const data = Object.fromEntries(headers.map((h, j) => [h, values[j].trim()]))
    if (!['receipt', 'purchase', 'expense', 'opening_pool', 'reconciliation', 'supplier_refund', 'supplier_loss'].includes(data.type)) throw new Error(`第 ${i + 2} 行类型无效`)
    if (!/^-?\d+(\.\d{1,8})?$/.test(data.amount_cny)) throw new Error(`第 ${i + 2} 行人民币金额无效`)
    if (data.idempotency_key.length < 8 || keys.has(data.idempotency_key)) throw new Error(`第 ${i + 2} 行幂等标识无效或重复`)
    keys.add(data.idempotency_key)
    if (!/(Z|[+-]\d{2}:\d{2})$/.test(data.occurred_at) || !Number.isFinite(Date.parse(data.occurred_at))) throw new Error(`第 ${i + 2} 行需带时区的 ISO 时间`)
    const payload: Record<string, unknown> = { amount_cny: data.amount_cny, notes: data.notes }
    for (const key of ['credits', 'starts_at', 'ends_at', 'category', 'entry_kind']) if (data[key]) payload[key] = data[key]
    for (const key of ['pool_id', 'account_id', 'group_id']) if (data[key]) {
      if (!/^\d+$/.test(data[key])) throw new Error(`第 ${i + 2} 行 ${key} 无效`)
      payload[key] = Number(data[key])
    }
    // Bulk imports record receipts only; issuing wallet credits requires the explicit form.
    return { idempotency_key: data.idempotency_key, type: data.type, occurred_at: new Date(data.occurred_at).toISOString(), user_id: Number(data.user_id || 0), payload }
  })
}
