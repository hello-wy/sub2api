import { describe, it, expect } from 'vitest'
import { plazaPrices, plazaV1Status, plazaV2Status, plazaHistory } from '@/utils/model-plaza'
import type { MonitorMatrixResponse } from '@/api/channelMonitorV2'
import type { UserMonitorView } from '@/api/channelMonitor'
import { group, model, matrixRow, metric } from './fixtures'

describe('plaza pricing', () => {
  it('applies personal rates only to paid prices and preserves zero/missing values', () => {
    const m = model()
    m.pricing!.input_price = 0
    expect(plazaPrices(m, group(), 'paid')[0]).toMatchObject({ input:0, output:4, cacheWrite:null })
    expect(plazaPrices(m, group(), 'official')[0]).toMatchObject({ input:3, output:12 })
  })
  it('resolves absolute and relative tiers with time multipliers and cache durations', () => {
    const m = model()
    m.pricing!.cache_write_1h_price = 4e-6
    m.pricing!.intervals = [{ min_tokens:200000, max_tokens:null, input_price:null, output_price:10e-6, cache_write_price:null, cache_read_price:null, per_request_price:null, input_multiplier:2, cache_write_multiplier:2 }]
    const result = plazaPrices(m, group(), 'paid', 1.5)[0]
    expect(result.input).toBeCloseTo(3)
    expect(result.output).toBeCloseTo(7.5)
    expect(result.cacheWrite1h).toBeCloseTo(6)
    expect(result.label).toBe('>200K')
  })
  it('uses independent image rates and keeps official token units independent', () => {
    const m = model()
    m.pricing!.billing_mode = 'image'
    m.pricing!.per_request_price = .2
    const g = group({ image_rate_independent:true, image_rate_multiplier:2 })
    expect(plazaPrices(m,g,'paid')[0]).toMatchObject({ request:.4, input:null, output:null })
    expect(plazaPrices(m,g,'official')[0]).toMatchObject({ request:null, input:3 })
  })
  it('retains all per-request tier prices and does not apply image overrides to requests', () => {
    const m = model()
    m.pricing!.billing_mode = 'per_request'
    m.pricing!.intervals = [1,2].map((n) => ({ min_tokens:n, max_tokens:n+1, tier_label:'Size '+n, input_price:null, output_price:null, cache_read_price:null, cache_write_price:null, per_request_price:n }))
    expect(plazaPrices(m,group({ image_rate_independent:true, image_rate_multiplier:9 }),'paid').map(row => row.request)).toEqual([.5,1])
  })
  it('does not fabricate prices when catalogs do not cover a model', () => {
    const m = model({ pricing:null, official_pricing:null })
    expect(plazaPrices(m,group(),'paid')[0].input).toBeNull()
    expect(plazaPrices(m,group(),'official')[0].input).toBeNull()
  })
})
describe('plaza monitor identity and units', () => {
  const matrix = (items = [matrixRow()]): MonitorMatrixResponse => ({
    group_by:'platform_group_model', items,
    coverage:{ requested_start:'2026-09-25T00:00:00Z', coverage_start:'2026-09-25T00:00:00Z', data_through:'2026-09-26T00:00:00Z', computed_at:'2026-09-26T00:00:01Z', aggregation_lag_seconds:1, coverage_complete:true, bucket_seconds:3600 },
  })
  it('matches group id, provider and model; uses total duration and TPM/60', () => {
    expect(plazaV2Status(model(),group(),matrix())).toMatchObject({ health:'healthy', availability:99, throughput:10, latency:1250, window:'24h' })
    expect(plazaV2Status(model(),group({ id:2 }),matrix()).health).toBe('unknown')
    expect(plazaV2Status(model({ platform:'anthropic' }),group(),matrix()).health).toBe('unknown')
    expect(plazaV2Status(model({ name:'another' }),group(),matrix()).health).toBe('unknown')
    expect(plazaV2Status(model(),group(),matrix([matrixRow(),matrixRow()])).health).toBe('unknown')
  })
  it('leaves rates unknown when there are no requests', () => {
    expect(plazaV2Status(model(),group(),matrix([matrixRow({ metrics:metric({ request_count:0 }) })]))).toMatchObject({ availability:null, throughput:null })
  })
  it('does not borrow ambiguous or ungrouped V1 probe results', () => {
    const probe: UserMonitorView = { id:1, name:'channel', provider:'openai', group_name:'OpenAI 标准', primary_model:'gpt-test', primary_status:'degraded', primary_latency_ms:120, primary_ping_latency_ms:10, primary_throughput_tps:20, availability_7d:98.2, extra_models:[], timeline:[{ status:'operational', latency_ms:100, ping_latency_ms:10, checked_at:'2026-09-26T00:00:00Z' }] }
    expect(plazaV1Status(model(),group(),[probe])).toMatchObject({ health:'warning', availability:98.2, throughput:20, window:'7d' })
    expect(plazaV1Status(model(),group(),[probe,probe]).health).toBe('unknown')
    expect(plazaV1Status(model(),group(),[{ ...probe, group_name:'' }]).health).toBe('unknown')
  })
})

describe('plaza history compression', () => {
  it('keeps a single failed bucket visible within a dense history', () => {
    const history = Array.from({ length:288 }, (_,i) => ({ health:i === 137 ? 'critical' as const : 'healthy' as const, at:String(i) }))
    const bars = plazaHistory(history)
    expect(bars).toHaveLength(24)
    expect(bars.filter(b => b.health === 'critical')).toHaveLength(1)
    expect(bars[0].at).toBe('0')
    expect(bars[23].endAt).toBe('287')
  })
})
