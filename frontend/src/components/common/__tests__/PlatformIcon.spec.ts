import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PlatformIcon from '../PlatformIcon.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { platformAccentColor } from '@/utils/platformColors'

describe('platform brand icons', () => {
  it.each(['openai', 'anthropic', 'gemini', 'grok', 'kimi', 'minimax', 'zhipu', 'deepseek'])(
    'shares the colored %s artwork between plaza and monitor', platform => {
      const plaza = mount(PlatformIcon, { props: { platform, colored: true } })
      const monitor = mount(ProviderIcon, { props: { provider: platform, size: 28 } })
      expect(plaza.get('svg').classes()).toContain('platform-icon-colored')
      expect(monitor.getComponent(PlatformIcon).props('colored')).toBe(true)
      expect(plaza.get('svg').attributes('style')).toContain(platformAccentColor(platform))
      expect(monitor.get('svg').attributes('style')).toContain('width: 28px')
      expect(monitor.findAll('path').map(path => path.attributes('d')))
        .toEqual(plaza.findAll('path').map(path => path.attributes('d')))
    },
  )
  it('preserves inherited colors for existing monochrome callers', () => {
    const wrapper = mount(PlatformIcon, { props: { platform: 'openai' } })
    expect(wrapper.get('svg').classes()).not.toContain('platform-icon-colored')
    expect(wrapper.get('svg').attributes('style')).toBeUndefined()
  })
  it('uses Kimi blue accents and MiniMax brand gradients', () => {
    const kimi = mount(PlatformIcon, { props: { platform: 'kimi', colored: true } })
    expect(kimi.get('path').attributes('fill')).toBe('#027AFF')
    const minimax = mount(PlatformIcon, { props: { platform: 'minimax', colored: true } })
    expect(minimax.findAll('stop').map(stop => stop.attributes('stop-color'))).toEqual(['#E2167E', '#FE603C'])
  })
  it('keeps gradient paint IDs unique when several icons share a page', () => {
    const wrapper = mount({ components: { PlatformIcon }, template: '<div><PlatformIcon platform="gemini" colored /><PlatformIcon platform="gemini" colored /><PlatformIcon platform="minimax" colored /><PlatformIcon platform="minimax" colored /></div>' })
    const ids = wrapper.findAll('linearGradient').map(gradient => gradient.attributes('id'))
    expect(new Set(ids).size).toBe(ids.length)
    for (const path of wrapper.findAll('path[fill^="url"]')) {
      expect(ids).toContain(path.attributes('fill').slice(5, -1))
    }
  })
})
