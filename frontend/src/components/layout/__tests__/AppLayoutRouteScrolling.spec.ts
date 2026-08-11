import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const scrollableRoutes = [
  'src/views/admin/UsageView.vue',
  'src/views/admin/SettingsView.vue',
  'src/views/admin/RiskControlView.vue',
  'src/views/admin/DashboardView.vue',
  'src/views/admin/orders/AdminOrdersView.vue',
  'src/views/admin/orders/AdminPaymentDashboardView.vue',
  'src/views/admin/orders/AdminPaymentPlansView.vue',
  'src/features/prompt-audit/PromptAuditView.vue',
  'src/views/ModelPlazaView.vue',
  'src/views/user/UsageView.vue',
  'src/views/user/DashboardView.vue',
  'src/views/user/PaymentView.vue',
  'src/views/user/ChannelStatusView.vue',
  'src/views/user/AffiliateView.vue',
  'src/views/user/SubscriptionsView.vue',
  'src/views/user/UserOrdersView.vue',
  'src/views/user/ProfileView.vue',
  'src/views/user/RedeemView.vue',
  'src/views/user/PaymentQRCodeView.vue',
  'src/views/user/AirwallexPaymentView.vue',
  'src/views/user/LoyaltyView.vue',
  'src/views/user/CheckinView.vue',
  'src/views/user/ModelSquareView.vue',
  'src/views/public/LeaderboardView.vue',
]

const readSource = (path: string) => readFileSync(resolve(process.cwd(), path), 'utf8')

describe('AppLayout route scroll ownership', () => {
  it.each(scrollableRoutes)('%s uses the shared route scroll shell', (path) => {
    const source = readSource(path)

    expect(source).toMatch(/import ScrollablePageLayout from ['"]@\/components\/layout\/ScrollablePageLayout\.vue['"]/)
    expect(source).toContain('<ScrollablePageLayout')
  })

  it('keeps Stripe popup mode separate while scrolling the AppLayout mode', () => {
    const source = readSource('src/views/user/StripePaymentView.vue')

    expect(source).toMatch(/import ScrollablePageLayout from ['"]@\/components\/layout\/ScrollablePageLayout\.vue['"]/)
    expect(source).toContain(":is=\"isPopup ? 'div' : ScrollablePageLayout\"")
  })

  it('removes page-level overflow suppression from full-bleed routes', () => {
    for (const path of [
      'src/views/user/LoyaltyView.vue',
      'src/views/user/CheckinView.vue',
      'src/views/user/ModelSquareView.vue',
    ]) {
      const source = readSource(path)
      const shellTag = source.match(/<ScrollablePageLayout[^>]*>/)?.[0] ?? ''

      expect(shellTag).not.toContain('overflow-hidden')
      expect(shellTag).not.toContain('min-h-[calc(100vh-4rem)]')
    }
  })
})
