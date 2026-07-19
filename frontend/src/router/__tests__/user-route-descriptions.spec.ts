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

  it('adds concise subtitles to payment pages that use the app header', () => {
    expect(routerSource).toMatch(
      /path: '\/payment\/qrcode'[\s\S]*descriptionKey: 'payment\.qr\.description'/,
    )
    expect(routerSource).toMatch(
      /path: '\/payment\/airwallex'[\s\S]*descriptionKey: 'payment\.airwallexDescription'/,
    )
    expect(routerSource).toMatch(
      /path: '\/admin\/orders\/dashboard'[\s\S]*descriptionKey: 'payment\.admin\.dashboardDescription'/,
    )
    expect(routerSource).toMatch(
      /path: '\/admin\/orders'[\s\S]*descriptionKey: 'payment\.admin\.ordersDescription'/,
    )
    expect(routerSource).toMatch(
      /path: '\/admin\/orders\/plans'[\s\S]*descriptionKey: 'payment\.admin\.plansDescription'/,
    )
    expect(zhMisc.payment.qr.description).toBe('扫码完成本次支付')
    expect(zhMisc.payment.airwallexDescription).toBe('安全完成本次在线支付')
    expect(zhMisc.payment.admin.dashboardDescription).toBe('查看支付收入与订单趋势')
    expect(zhMisc.payment.admin.ordersDescription).toBe('管理充值与订阅订单')
    expect(zhMisc.payment.admin.plansDescription).toBe('配置可售订阅套餐')
  })
})
