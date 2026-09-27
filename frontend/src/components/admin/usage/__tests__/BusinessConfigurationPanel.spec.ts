import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BusinessConfigurationPanel from '../BusinessConfigurationPanel.vue'
import type { BusinessConfiguration } from '@/api/admin/business'

const api = vi.hoisted(() => ({ entities: vi.fn(), pool: vi.fn(), updatePool: vi.fn(), bindAccounts: vi.fn(), rule: vi.fn() }))
vi.mock('@/api/admin/business', () => ({ businessAPI: api }))
const Dialog = defineComponent({ props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' })
// Exercise the entity selector's option state as well as the parent form.
const Select = defineComponent({
  props: ['modelValue', 'options', 'ariaLabel'], emits: ['update:modelValue', 'search'],
  template: '<select :aria-label="ariaLabel" :value="modelValue" @change="$emit(\'update:modelValue\', Number($event.target.value))"><option value="0">选择</option><option v-for="o in options" :key="o.value" :value="o.value" :disabled="o.disabled">{{ o.label }}</option></select>',
})
const configuration = (): BusinessConfiguration => ({ enabled_at: '2026-09-01T00:00:00Z', timezone: 'Asia/Shanghai', pools: [{ id: 1, name: '供应商 A', supplier: 'A', mode: 'postpaid', unit: 'CNY' }, { id: 2, name: '供应商 B', supplier: 'B', mode: 'prepaid', unit: 'USD' }], bindings: [{ id: 1, account_id: 1, account_name: '已有账号', pool_id: 1, effective_at: '2026-09-01T00:00:00Z' }], rules: [] })
function render() { return mount(BusinessConfigurationPanel, { props: { configuration: configuration() }, global: { stubs: { BaseDialog: Dialog, Select } } }) }
async function bind(wrapper: ReturnType<typeof render>) { await wrapper.get('section[aria-label="账号绑定历史"] button').trigger('click'); await wrapper.get('select[aria-label="成本池"]').setValue(1); await flushPromises() }
beforeEach(() => { vi.clearAllMocks(); api.entities.mockResolvedValue([{ id: 1, name: '已有账号', kind: 'accounts' }, { id: 2, name: '新账号', kind: 'accounts' }]); api.bindAccounts.mockResolvedValue({ count: 1 }); api.updatePool.mockResolvedValue({}) })
describe('经营配置', () => {
  it('成本池、绑定和价格历史各自展示为独立模块', () => {
    const wrapper = render()
    expect(wrapper.findAll('section[aria-label]').map(s => s.attributes('aria-label'))).toEqual(['成本池', '账号绑定历史', '价格规则历史'])
  })
  it('选择后禁用重复选项，移除后可重新选择，提交时记住已绑定账号', async () => {
    const wrapper = render(); await bind(wrapper)
    const select = () => wrapper.get('select[aria-label="搜索账号名称或 ID"]')
    expect(select().get('option[value="1"]').attributes('disabled')).toBeDefined()
    expect(select().get('option[value="1"]').text()).toContain('已绑定当前成本池')
    await select().setValue(2)
    expect(select().get('option[value="2"]').attributes('disabled')).toBeDefined()
    expect(select().get('option[value="2"]').text()).toContain('本次已选')
    await wrapper.get('button[aria-label="移除账号 2"]').trigger('click')
    expect(select().get('option[value="2"]').attributes('disabled')).toBeUndefined()
    await select().setValue(2)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.bindAccounts).toHaveBeenCalledWith(expect.objectContaining({ account_ids: [2], pool_id: 1 }))
    await bind(wrapper)
    expect(select().get('option[value="2"]').text()).toContain('已绑定当前成本池')
    expect(select().get('option[value="2"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('切换成本池允许迁移绑定，历史日期按当时的绑定识别', async () => {
    const wrapper = render(); await bind(wrapper)
    const option = () => wrapper.get('select[aria-label="搜索账号名称或 ID"] option[value="1"]')
    await wrapper.get('select[aria-label="成本池"]').setValue(2)
    expect(option().attributes('disabled')).toBeUndefined()
    expect(option().text()).toContain('已绑定 供应商 A')
    await wrapper.get('select[aria-label="成本池"]').setValue(1)
    await wrapper.get('input[aria-label="生效时间"]').setValue('2026-08-01T00:00')
    expect(option().attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('搜索结果更新仍保留已选择账号与禁用状态', async () => {
    const wrapper = render(); await bind(wrapper)
    const entity = wrapper.findComponent({ name: 'BusinessEntitySelect' })
    await wrapper.get('select[aria-label="搜索账号名称或 ID"]').setValue(2)
    api.entities.mockResolvedValueOnce([{ id: 1, name: '已有账号' }])
    entity.findComponent(Select).vm.$emit('search', '已有'); await flushPromises()
    expect(wrapper.get('button[aria-label="移除账号 2"]').text()).toContain('新账号')
    api.entities.mockResolvedValueOnce([{ id: 2, name: '新账号' }])
    entity.findComponent(Select).vm.$emit('search', '新'); await flushPromises()
    expect(wrapper.get('select[aria-label="搜索账号名称或 ID"] option[value="2"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('编辑已有成本池调用更新接口并保护已使用的计价定义', async () => {
    const wrapper = render()
    await wrapper.get('button[aria-label="编辑成本池 供应商 A"]').trigger('click')
    expect(wrapper.get('input[aria-label="成本池名称"]').element.value).toBe('供应商 A')
    expect(wrapper.get('select[aria-label="成本方式"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('select[aria-label="上游额度单位"]').attributes('disabled')).toBeDefined()
    await wrapper.get('input[aria-label="成本池名称"]').setValue('更正名称')
    api.updatePool.mockRejectedValueOnce(new Error('保存失败'))
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.text()).toContain('保存失败')
    expect(wrapper.get('input[aria-label="成本池名称"]').element.value).toBe('更正名称')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.updatePool).toHaveBeenLastCalledWith(1, { name: '更正名称', supplier: 'A', mode: 'postpaid', unit: 'CNY' })
    expect(api.pool).not.toHaveBeenCalled()
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })
})
