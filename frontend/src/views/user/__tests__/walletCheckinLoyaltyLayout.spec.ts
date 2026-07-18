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

  it('keeps the VIP artwork free of a framing surface', () => {
    expect(loyaltySource).toContain('border: 0;')
    expect(loyaltySource).toContain('background: transparent;')
    expect(loyaltySource).toContain('box-shadow: none;')
  })

  it('uses a container-aware tier grid for both membership plans', () => {
    expect(loyaltySource).toContain('class="loyalty-tier-grid mt-5"')
    expect(loyaltySource).toContain('grid-template-columns: repeat(4, minmax(0, 1fr));')
    expect(loyaltySource).toContain('class="loyalty-plan-grid grid gap-5 md:grid-cols-2"')
    expect(loyaltySource).toContain(':class="`loyalty-plan-${plan.key}`"')
    expect(loyaltySource).toContain('.loyalty-plan-permanent .loyalty-tier-grid')
    expect(loyaltySource).toContain('grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.85fr);')
    expect(loyaltySource).not.toContain("'lg:grid-cols-4'")
    expect(loyaltySource).not.toContain('class="grid gap-5 2xl:grid-cols-2"')
  })

  it('keeps membership surfaces solid instead of liquid glass', () => {
    expect(loyaltySource).toContain('background: #ffffff;')
    expect(loyaltySource).toContain('box-shadow: none;')
    expect(loyaltySource).not.toContain('backdrop-filter:')
    expect(loyaltySource).not.toContain('background: rgba(255, 255, 255, 0.94);')
  })
})
