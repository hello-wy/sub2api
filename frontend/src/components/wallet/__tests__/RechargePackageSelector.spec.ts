import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import RechargePackageSelector from '../RechargePackageSelector.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('RechargePackageSelector', () => {
  it('converts a fixed credited balance into the payment amount', async () => {
    const wrapper = mount(RechargePackageSelector, {
      props: {
        modelValue: null,
        credits: [10],
        multiplier: 0.1,
        formatAmount: (value: number) => `¥${value.toFixed(2)}`,
      },
    })

    const fixedPackage = wrapper.findAll('button').find((button) => button.text().includes('$10'))
    expect(fixedPackage).toBeTruthy()
    expect(fixedPackage?.text()).toContain('¥100.00')

    await fixedPackage?.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([100])
  })

  it('supports a custom credited balance with the same conversion rate', async () => {
    const wrapper = mount(RechargePackageSelector, {
      props: {
        modelValue: null,
        credits: [10],
        multiplier: 0.1,
        formatAmount: (value: number) => `¥${value.toFixed(2)}`,
      },
    })

    const customButton = wrapper.findAll('button').find((button) => button.text().includes('wallet.customRecharge'))
    await customButton?.trigger('click')
    await wrapper.get('input').setValue('25')

    expect(wrapper.text()).toContain('¥250.00')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([250])

    const emitCount = wrapper.emitted('update:modelValue')?.length
    await wrapper.get('input').setValue('invalid')
    expect(wrapper.get('input').element.value).toBe('25')
    expect(wrapper.emitted('update:modelValue')?.length).toBe(emitCount)
  })
})
