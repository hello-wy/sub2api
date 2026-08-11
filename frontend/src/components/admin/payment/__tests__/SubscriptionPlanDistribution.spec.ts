import { describe, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import SubscriptionPlanDistribution from '../SubscriptionPlanDistribution.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('SubscriptionPlanDistribution', () => {
  it('renders each subscription plan purchase count and its relative bar width', () => {
    const wrapper = shallowMount(SubscriptionPlanDistribution, {
      props: {
        plans: [
          { plan_id: 1, plan_name: '专业版', count: 8 },
          { plan_id: 2, plan_name: '团队版', count: 4 },
        ],
      },
    })

    expect(wrapper.text()).toContain('专业版')
    expect(wrapper.text()).toContain('团队版')
    expect(wrapper.text()).toContain('payment.admin.subscriptionPlanDistribution')
    expect(wrapper.text()).toContain('8')
    expect(wrapper.findAll('.bg-cyan-500')[1].attributes('style')).toContain('width: 50%')
  })

  it('identifies historical orders whose subscription plan no longer exists', () => {
    const wrapper = shallowMount(SubscriptionPlanDistribution, {
      props: { plans: [{ plan_id: 9, plan_name: '', count: 1 }] },
    })

    expect(wrapper.text()).toContain('payment.admin.unknownSubscriptionPlan #9')
  })
})
