import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

const WELFARE_KEYS = [
  'admin.welfare.title',
  'admin.welfare.description',
  'admin.welfare.searchPlaceholder',
  'admin.welfare.type.all',
  'admin.welfare.type.leaderboard',
  'admin.welfare.type.checkin',
  'admin.welfare.statusFilter.all',
  'admin.welfare.status.success',
  'admin.welfare.status.revoked',
  'admin.welfare.dashboard.totalRewards',
  'admin.welfare.dashboard.totalAmount',
  'admin.welfare.dashboard.breakdown',
  'admin.welfare.table.email',
  'admin.welfare.table.amount',
  'admin.welfare.table.type',
  'admin.welfare.table.remarks',
  'admin.welfare.table.status',
  'admin.welfare.table.createdAt',
  'admin.welfare.table.actions',
  'admin.welfare.action.revoke',
  'admin.welfare.action.revokeButton',
  'admin.welfare.action.revokeConfirmTitle',
  'admin.welfare.action.revokeConfirmMessage',
  'admin.welfare.action.revokeSuccess'
] as const

function readMessage(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((value, key) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return undefined
    }
    return (value as LocaleMessages)[key]
  }, messages)
}

describe.each([
  ['en', en],
  ['zh', zh]
] as const)('%s welfare locales', (locale, messages) => {
  it.each(WELFARE_KEYS)('defines %s', (key) => {
    expect(readMessage(messages, key)).toEqual(expect.any(String))
  })
})

it('uses Chinese copy for the welfare records page', () => {
  expect(readMessage(zh, 'admin.welfare.table.email')).toBe('用户邮箱')
  expect(readMessage(zh, 'admin.welfare.action.revokeButton')).toBe('撤销发放')
})
