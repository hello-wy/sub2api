import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import GroupSelector from '../GroupSelector.vue'

const authState = { isSimpleMode: false }

vi.mock('@/stores', () => ({ useAuthStore: () => authState }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const groups = [
  { id: 1, name: 'Basic', platform: 'anthropic', status: 'active' },
  { id: 2, name: 'Composite', platform: 'composite', status: 'active' }
] as any

const mountSelector = (modelValue: number[] = []) => mount(GroupSelector, {
  props: { modelValue, groups },
  global: { stubs: { GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' }, Icon: true } }
})

const taggedGroups = [
  { id: 1, name: 'OpenAI A', tag: 'production', platform: 'openai', status: 'active' },
  { id: 2, name: 'OpenAI B', tag: 'production', platform: 'openai', status: 'active' },
  { id: 3, name: 'Claude', tag: 'production', platform: 'anthropic', status: 'active' },
  { id: 4, name: 'OpenAI Backup', tag: 'backup', platform: 'openai', status: 'active' }
] as any

describe('GroupSelector simple-mode binding policy', () => {
  beforeEach(() => { authState.isSimpleMode = false })

  it('hides composite groups in simple mode and preserves basic groups', () => {
    authState.isSimpleMode = true
    const wrapper = mountSelector()
    expect(wrapper.text()).toContain('Basic')
    expect(wrapper.text()).not.toContain('Composite')
  })

  it('keeps composite groups available in advanced mode', () => {
    const wrapper = mountSelector()
    expect(wrapper.text()).toContain('Composite')
  })

  it('cleans hidden historical composite IDs while preserving visible selections', () => {
    authState.isSimpleMode = true
    const wrapper = mountSelector([1, 2])
    expect(wrapper.emitted('update:modelValue')).toEqual([[[1]]])
  })
})

describe('GroupSelector tag selection', () => {
  beforeEach(() => { authState.isSimpleMode = false })

  it('selects and then clears all eligible groups with the same tag', async () => {
    const wrapper = mount(GroupSelector, {
      props: { modelValue: [4], groups: taggedGroups, platform: 'openai' },
      global: { stubs: { GroupBadge: true, Icon: true } }
    })
    const button = wrapper.get('[data-group-tag="production"]')

    await button.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[4, 1, 2]])

    await wrapper.setProps({ modelValue: [4, 1, 2] })
    await button.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual([[4]])
  })

  it('does not select platform-incompatible groups that share the tag', async () => {
    const wrapper = mount(GroupSelector, {
      props: { modelValue: [], groups: taggedGroups, platform: 'openai' },
      global: { stubs: { GroupBadge: true, Icon: true } }
    })

    await wrapper.get('[data-group-tag="production"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[1, 2]])
  })

  it('matches groups by tag when searching', async () => {
    const wrapper = mount(GroupSelector, {
      props: { modelValue: [], groups: taggedGroups, platform: 'openai', searchable: true },
      global: {
        stubs: {
          GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' },
          Icon: true
        }
      }
    })

    await wrapper.get('input[type="text"]').setValue('backup')
    expect(wrapper.text()).toContain('OpenAI Backup')
    expect(wrapper.text()).not.toContain('OpenAI A')
  })
})
