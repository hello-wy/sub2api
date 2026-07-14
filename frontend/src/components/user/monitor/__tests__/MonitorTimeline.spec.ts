import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MonitorTimeline from '../MonitorTimeline.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.n === undefined ? key : `${key}:${params.n}`,
    }),
  }
})

describe('MonitorTimeline', () => {
  it('shows the next-update countdown by default for MonitorCard compatibility', () => {
    const wrapper = mount(MonitorTimeline, {
      props: { countdownSeconds: 12 },
    })

    expect(wrapper.text()).toContain('monitorCommon.nextUpdateIn:12')
  })

  it('keeps all timeline bars inside a shrinkable clipped track', () => {
    const wrapper = mount(MonitorTimeline, {
      props: { countdownSeconds: 12 },
    })
    const track = wrapper.get('[data-test="timeline-track"]')
    const bars = track.findAll('div')

    expect(bars).toHaveLength(60)
    expect(track.classes()).toEqual(expect.arrayContaining(['min-w-0', 'overflow-hidden']))
    expect(bars.every(bar => bar.classes().includes('min-w-0'))).toBe(true)
    expect(bars.some(bar => bar.classes().includes('min-w-[3px]'))).toBe(false)
  })

  it('hides the next-update countdown when requested', () => {
    const wrapper = mount(MonitorTimeline, {
      props: {
        countdownSeconds: 12,
        showCountdown: false,
      },
    })

    expect(wrapper.text()).not.toContain('monitorCommon.nextUpdateIn')
    expect(wrapper.text()).toContain('monitorCommon.history60pts:60')
  })
})
