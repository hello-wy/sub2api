import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import Select from '../Select.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
let wrapper: ReturnType<typeof mount>
afterEach(() => { wrapper?.unmount(); document.body.innerHTML = '' })

function setup() {
  wrapper = mount({
    components: { Select },
    template: `<div @click.stop><Select :model-value="null" :options="[]" /><Select :model-value="null" :options="[]" /><span data-blank>Blank</span></div>`
  }, { attachTo: document.body })
  return wrapper.findAllComponents(Select)
}

describe('Select inside a dialog with stopped click propagation', () => {
  it('closes the previous dropdown on mouse and keyboard opening, and on blank clicks', async () => {
    const selects = setup()
    await selects[0].get('button').trigger('click')
    await selects[1].get('button').trigger('click')
    expect(selects.map(s => s.get('button').attributes('aria-expanded'))).toEqual(['false', 'true'])
    await selects[0].get('button').trigger('keydown', { key: 'ArrowDown' })
    await nextTick()
    expect(selects.map(s => s.get('button').attributes('aria-expanded'))).toEqual(['true', 'false'])
    await wrapper.get('[data-blank]').trigger('click')
    expect(selects[0].get('button').attributes('aria-expanded')).toBe('false')
  })

  it('consumes Escape before it can close the parent dialog', async () => {
    const selects = setup()
    await selects[0].get('button').trigger('click')
    const listener = vi.fn()
    document.addEventListener('keydown', listener)
    document.querySelector('[role="listbox"]')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    document.removeEventListener('keydown', listener)
    expect(listener).not.toHaveBeenCalled()
    expect(selects[0].get('button').attributes('aria-expanded')).toBe('false')
  })
})
