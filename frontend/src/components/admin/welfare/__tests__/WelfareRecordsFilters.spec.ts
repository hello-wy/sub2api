import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

import WelfareRecordsFilters from '../WelfareRecordsFilters.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const WelfareSelectStub = defineComponent({
  name: 'WelfareSelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: null
    },
    options: {
      type: Array,
      required: true
    }
  },
  emits: ['update:modelValue', 'change'],
  template: '<div class="select-stub" />'
})

function mountFilters() {
  return mount(WelfareRecordsFilters, {
    props: {
      search: '',
      type: '',
      status: '',
      startDate: '2026-06-29',
      endDate: '2026-06-30',
      loading: false
    },
    global: {
      stubs: {
        DateRangePicker: true,
        Icon: true,
        Select: WelfareSelectStub
      }
    }
  })
}

describe('WelfareRecordsFilters', () => {
  it('uses shared Select controls for benefit type and status filters', async () => {
    const wrapper = mountFilters()
    const selects = wrapper.findAllComponents(WelfareSelectStub)

    expect(selects).toHaveLength(2)
    expect(wrapper.findAll('select')).toHaveLength(0)
    expect(selects[0].props('options')).toEqual([
      { value: '', label: 'admin.welfare.type.all' },
      { value: 'leaderboard', label: 'admin.welfare.type.leaderboard' },
      { value: 'checkin', label: 'admin.welfare.type.checkin' },
      { value: 'lottery', label: 'admin.welfare.type.lottery' }
    ])
    expect(selects[1].props('options')).toEqual([
      { value: '', label: 'admin.welfare.statusFilter.all' },
      { value: 'success', label: 'admin.welfare.status.success' },
      { value: 'revoked', label: 'admin.welfare.status.revoked' }
    ])

    await selects[0].vm.$emit('update:modelValue', 'checkin')
    await selects[0].vm.$emit('change')
    await selects[1].vm.$emit('update:modelValue', 'revoked')
    await selects[1].vm.$emit('change')

    expect(wrapper.emitted('update:type')?.[0]).toEqual(['checkin'])
    expect(wrapper.emitted('type-change')).toHaveLength(1)
    expect(wrapper.emitted('update:status')?.[0]).toEqual(['revoked'])
    expect(wrapper.emitted('status-change')).toHaveLength(1)
  })
})
