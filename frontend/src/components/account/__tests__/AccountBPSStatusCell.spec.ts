import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountBPSStatusCell from '../AccountBPSStatusCell.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const oauthAccount = { platform: 'openai', type: 'oauth' } as const

describe('AccountBPSStatusCell', () => {
  it.each([
    ['enabled', 'badge-success'],
    ['disabled', 'badge-gray'],
    ['disabled_403', 'badge-warning']
  ] as const)('renders the %s status', (status, badgeClass) => {
    const wrapper = mount(AccountBPSStatusCell, { props: { account: { ...oauthAccount, excel_bps_status: status } } })
    const badge = wrapper.get('[data-testid="bps-status"]')
    expect(badge.text()).toBe(`admin.accounts.bpsStatus.${status}`)
    expect(badge.classes()).toContain(badgeClass)
  })

  it('shows the recorded 403 time and updates after BPS is reopened', async () => {
    const disabledAt = '2026-09-27T01:02:03Z'
    const extra = { openai_excel_bps_disabled_at: disabledAt }
    const wrapper = mount(AccountBPSStatusCell, {
      props: { account: { ...oauthAccount, excel_bps_status: 'disabled_403', extra } }
    })
    expect(wrapper.get('time').attributes('datetime')).toBe(disabledAt)
    expect(wrapper.get('[data-testid="bps-status"]').attributes('title')).toBe('admin.accounts.bpsStatus.autoDisabledHint')
    await wrapper.setProps({ account: { ...oauthAccount, excel_bps_status: 'enabled', extra } })
    expect(wrapper.find('time').exists()).toBe(false)
    expect(wrapper.get('[data-testid="bps-status"]').text()).toBe('admin.accounts.bpsStatus.enabled')
  })

  it('does not infer automatic closure from the opt-in setting', () => {
    const wrapper = mount(AccountBPSStatusCell, { props: { account: {
      ...oauthAccount,
      excel_bps_status: 'disabled',
      extra: { openai_excel_bps_auto_disable_on_403: true }
    } } })
    expect(wrapper.get('[data-testid="bps-status"]').text()).toBe('admin.accounts.bpsStatus.disabled')
  })

  it.each([
    ['other platform', { platform: 'anthropic' }],
    ['API key', { type: 'apikey' }],
    ['shadow', { parent_account_id: 18 }],
    ['Agent Identity', { credentials: { auth_mode: ' AgentIdentity ' } }],
    ['PAT', { credentials: { auth_mode: 'personalAccessToken' } }],
    ['legacy PAT', { credentials: { openai_auth_mode: ' personal_access_token ' } }]
  ] satisfies [string, Partial<Account>][])('explains that %s accounts are not applicable', (_name, overrides) => {
    const wrapper = mount(AccountBPSStatusCell, { props: { account: { ...oauthAccount, ...overrides } } })
    const label = wrapper.get('[data-testid="bps-not-applicable"]')
    expect(label.text()).toBe('admin.accounts.bpsStatus.notApplicable')
    expect(label.attributes('title')).toBe('admin.accounts.bpsStatus.notApplicableHint')
    expect(wrapper.find('[data-testid="bps-status"]').exists()).toBe(false)
  })

  it.each([
    ['enabled switch', { openai_excel_bps: true }, 'enabled'],
    ['disabled switch', { openai_excel_bps: false }, 'disabled'],
    ['default disabled', undefined, 'disabled'],
    ['403 opt-in alone', { openai_excel_bps: false, openai_excel_bps_auto_disable_on_403: true }, 'disabled'],
    ['recorded 403', { openai_excel_bps: false, openai_excel_bps_disabled_reason: 'http_403' }, 'disabled_403'],
    ['reopened switch', { openai_excel_bps: true, openai_excel_bps_disabled_reason: 'http_403' }, 'enabled']
  ] satisfies [string, Account['extra'], string][])('handles a legacy response with %s', (_name, extra, expected) => {
    const wrapper = mount(AccountBPSStatusCell, { props: { account: { ...oauthAccount, extra } } })
    expect(wrapper.get('[data-testid="bps-status"]').text()).toBe(`admin.accounts.bpsStatus.${expected}`)
    expect(wrapper.find('[data-testid="bps-not-applicable"]').exists()).toBe(false)
  })

  it('prefers the explicit server status over fallback metadata', () => {
    const wrapper = mount(AccountBPSStatusCell, { props: { account: {
      ...oauthAccount, excel_bps_status: 'disabled', extra: { openai_excel_bps: true }
    } } })
    expect(wrapper.get('[data-testid="bps-status"]').text()).toBe('admin.accounts.bpsStatus.disabled')
  })

  it('omits an invalid automatic closure timestamp', () => {
    const wrapper = mount(AccountBPSStatusCell, { props: { account: {
      ...oauthAccount,
      excel_bps_status: 'disabled_403',
      extra: { openai_excel_bps_disabled_at: 'invalid timestamp' }
    } } })
    expect(wrapper.find('time').exists()).toBe(false)
    expect(wrapper.text()).toBe('admin.accounts.bpsStatus.disabled_403')
  })
})
