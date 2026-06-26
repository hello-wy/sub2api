import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

describe('admin welfare API', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  it('passes date range filters when listing records', async () => {
    const { apiClient } = await import('@/api/client')
    const { list } = await import('@/api/admin/welfare')
    const adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: { code: 0, data: { items: [], total: 0 } },
      headers: {},
      config: {},
      statusText: 'OK',
    })
    apiClient.defaults.adapter = adapter

    await list(2, 50, 'user@example.com', {
      startDate: '2026-06-25',
      endDate: '2026-06-26',
    })

    expect(adapter.mock.calls[0][0].params).toEqual(
      expect.objectContaining({
        page: 2,
        page_size: 50,
        email: 'user@example.com',
        start_date: '2026-06-25',
        end_date: '2026-06-26',
      })
    )
  })
})
