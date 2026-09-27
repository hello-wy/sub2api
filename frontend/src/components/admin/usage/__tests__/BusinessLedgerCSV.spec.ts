import { describe, it, expect } from 'vitest'
import { parseLedgerCSV, cny } from '@/utils/business-ledger'
describe('经营台账 CSV', () => {
  const header = 'idempotency_key,type,occurred_at,amount_cny,notes'
  it('保留十进制精度、人民币文本与带引号的多行凭据', () => {
    const rows = parseLedgerCSV('\uFEFF' + header + '\r\nreceipt-001,receipt,2026-08-01T12:30:00+08:00,123.12345678,"线下,收款\n订单""A"""\r\n')
    expect(rows[0].payload.amount_cny).toBe('123.12345678')
    expect(rows[0].payload.notes).toBe('线下,收款\n订单"A"')
    expect(rows[0].occurred_at).toBe('2026-08-01T04:30:00.000Z')
    expect(rows[0].payload.apply_balance).toBeUndefined()
  })
  it('拒绝重复标识、无时区日期和不完整数据', () => {
    const line = 'receipt-001,receipt,2026-08-01T00:00:00Z,100,收款'
    expect(() => parseLedgerCSV(header + '\n' + line + '\n' + line)).toThrow('重复')
    expect(() => parseLedgerCSV(header + '\nreceipt-001,receipt,2026-08-01,100,收款')).toThrow('时区')
    expect(() => parseLedgerCSV(header + '\nreceipt-001,receipt,2026-08-01T00:00:00Z,1e5,收款')).toThrow('金额')
    expect(() => parseLedgerCSV(header + '\nreceipt-001,receipt,2026-08-01T00:00:00Z,1,"收款')).toThrow('引号')
  })
  it('缺失金额不能呈现为零', () => {
    expect(cny(null)).toBe('待核对')
    expect(cny('0')).toBe('¥0.00')
  })
})
