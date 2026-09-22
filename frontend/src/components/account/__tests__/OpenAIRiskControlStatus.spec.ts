import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import OpenAIRiskControlStatus from '../OpenAIRiskControlStatus.vue'
import type { AccountListItem, OpenAIRiskControlStatus as RiskStatus } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

function makeAccount(status?: RiskStatus): AccountListItem {
  return {
    id: 1,
    name: 'openai-oauth',
    platform: 'openai',
    type: 'oauth',
    parent_account_id: null,
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-09-22T00:00:00Z',
    updated_at: '2026-09-22T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...(status ? {
      extra: {
        openai_risk_control: {
          status,
          suspected: status === 'suspected',
          ...(status === 'missing' ? {} : { state_length: status === 'normal' ? 292 : status === 'suspected' ? 312 : 356 }),
          http_status: 200,
          checked_at: '2026-09-22T08:00:00Z',
          reason: status === 'normal'
            ? 'turn_state_normal_length'
            : status === 'suspected'
              ? 'turn_state_length_312'
              : status === 'abnormal'
                ? 'turn_state_abnormal_length'
                : 'turn_state_missing'
        }
      }
    } : {})
  }
}

describe('OpenAIRiskControlStatus', () => {
  it.each([
    ['normal', 'admin.accounts.riskControl.normal'],
    ['suspected', 'admin.accounts.riskControl.suspected'],
    ['abnormal', 'admin.accounts.riskControl.abnormal'],
    ['missing', 'admin.accounts.riskControl.missing']
  ] as const)('独立显示 %s 风控状态', (status, labelKey) => {
    const wrapper = mount(OpenAIRiskControlStatus, {
      props: { account: makeAccount(status) },
      global: { stubs: { Icon: true } }
    })

    const badge = wrapper.get('[data-testid="openai-risk-control-status"]')
    expect(badge.attributes('data-status')).toBe(status)
    expect(badge.text()).toContain(labelKey)
  })

  it('尚未检测时显示独立的未检测状态', () => {
    const wrapper = mount(OpenAIRiskControlStatus, {
      props: { account: makeAccount() },
      global: { stubs: { Icon: true } }
    })
    expect(wrapper.get('[data-testid="openai-risk-control-status"]').text()).toContain(
      'admin.accounts.riskControl.unchecked'
    )
  })

  it('不支持检测的账号显示占位符', () => {
    const account = makeAccount()
    account.type = 'apikey'
    const wrapper = mount(OpenAIRiskControlStatus, { props: { account } })
    expect(wrapper.text()).toBe('-')
    expect(wrapper.find('[data-testid="openai-risk-control-status"]').exists()).toBe(false)
  })
})
