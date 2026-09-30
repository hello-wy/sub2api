import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import TicketRebateSettings from '../TicketRebateSettings.vue'

describe('TicketRebateSettings', () => {
  it('edits, adds and removes rules while keeping one rule', async () => {
    const wrapper = mount(TicketRebateSettings, {
      props: { enabled: false, rules: [{ amount_threshold: 5, ticket_count: 1 }] },
    })

    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.emitted('update:enabled')?.[0]).toEqual([true])

    await wrapper.get('button.btn-secondary').trigger('click')
    const added = wrapper.emitted('update:rules')?.[0]?.[0]
    expect(added).toEqual([
      { amount_threshold: 5, ticket_count: 1 },
      { amount_threshold: 10, ticket_count: 1 },
    ])

    await wrapper.setProps({ rules: added })
    await wrapper.get('button[title="删除规则"]').trigger('click')
    expect(wrapper.emitted('update:rules')?.[1]?.[0]).toEqual([
      { amount_threshold: 10, ticket_count: 1 },
    ])
  })
})
