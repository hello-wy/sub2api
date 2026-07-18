import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDirectory = dirname(fileURLToPath(import.meta.url))
const checkinSource = readFileSync(resolve(testDirectory, '../CheckinView.vue'), 'utf8')
const loyaltySource = readFileSync(resolve(testDirectory, '../LoyaltyView.vue'), 'utf8')
const paymentSource = readFileSync(resolve(testDirectory, '../PaymentView.vue'), 'utf8')

describe('wallet, check-in, and loyalty layout cleanup', () => {
  it('uses the provided cutout only for the check-in page hero', () => {
    expect(checkinSource).toContain("import checkinCalendarIcon from '@/assets/checkin-calendar.png'")
    expect(checkinSource).toContain(':src="checkinCalendarIcon"')
    expect(existsSync(resolve(testDirectory, '../../../assets/checkin-calendar.png'))).toBe(true)
  })

  it('removes decorative icons and localizes the reward label', () => {
    expect(checkinSource).not.toContain('card.icon')
    expect(checkinSource).not.toContain('card.accentIcon')
    expect(checkinSource).not.toContain('rewardRuleIcon')
    expect(checkinSource).not.toContain('Extra')
    expect(checkinSource).toContain('额外奖励')
  })

  it('removes the subscription count and loyalty explanation action', () => {
    expect(paymentSource).not.toContain('{{ activeSubscriptions.length }}')
    expect(loyaltySource).not.toContain('loyalty.viewRules')
    expect(loyaltySource).not.toContain('loyalty-table-action')
  })

  it('uses the VIP image in the membership hero and removes stat card icons', () => {
    expect(loyaltySource).toContain('membership-vip.png')
    expect(existsSync(resolve(testDirectory, '../../../assets/membership-vip.png'))).toBe(true)
    expect(loyaltySource).not.toContain(':name="card.icon"')
  })
})
