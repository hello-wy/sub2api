import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import zhDashboard from '@/i18n/locales/zh/dashboard'
import zhMisc from '@/i18n/locales/zh/misc'

const routerSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'),
  'utf8',
)

describe('user route subtitles', () => {
  it('connects the three user pages to header description keys', () => {
    expect(routerSource).toMatch(
      /path: '\/leaderboard'[\s\S]*descriptionKey: 'leaderboard\.description'/,
    )
    expect(routerSource).toMatch(
      /path: '\/monitor'[\s\S]*descriptionKey: 'channelStatus\.description'/,
    )
    expect(routerSource).toMatch(
      /path: '\/orders'[\s\S]*descriptionKey: 'payment\.orders\.description'/,
    )
  })

  it('provides concise Chinese subtitles for each page', () => {
    expect(zhDashboard.leaderboard.description).toBe('查看每日消费排行与个人排名')
    expect(zhDashboard.channelStatus.description).toBe('查看渠道可用性、延迟和近期状态')
    expect(zhMisc.payment.orders.description).toBe('查看充值与订阅订单记录')
  })
})
