import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import type { MonitorMatrixRow, MonitorMetric } from '@/api/channelMonitorV2'

export function model(overrides: Partial<PlazaModel> = {}): PlazaModel {
  return {
    name:'gpt-test', platform:'openai',
    pricing:{ billing_mode:'token', input_price:2e-6, output_price:8e-6, cache_read_price:.2e-6, cache_write_price:null, image_input_price:null, image_output_price:null, per_request_price:null, intervals:[] },
    official_pricing:{ input_price:3e-6, output_price:12e-6, cache_read_price:.3e-6, cache_write_price:null },
    ...overrides,
  }
}
export function group(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id:1, name:'OpenAI 标准', description:'', platform:'openai', subscription_type:'standard',
    rate_multiplier:.8, user_rate_multiplier:.5, peak_rate_enabled:false, peak_start:'', peak_end:'', peak_rate_multiplier:1,
    is_exclusive:false, image_rate_independent:false, image_rate_multiplier:1, long_context_pricing_enabled:true,
    models:[model()], ...overrides,
  }
}
export function metric(overrides: Partial<MonitorMetric> = {}): MonitorMetric {
  const latency = { sample_count:100, p50_ms:1250, p95_ms:2000, avg_ms:1500 }
  return { success_requests:99, error_requests:1, request_count:100, token_count:6000, rpm:10, tpm:600, error_rate:.01, cache_rate:0, cache_rate_numerator:0, cache_rate_denominator:0, ttft:latency, duration:latency, ...overrides }
}
export function matrixRow(overrides: Partial<MonitorMatrixRow> = {}): MonitorMatrixRow {
  return {
    platform:'openai', group_id:1, group_name:'OpenAI 标准', model:'gpt-test', metrics:metric(),
    health:{ overall:'healthy', error_rate:'healthy', ttft:'healthy', minimum_sample:10 }, buckets:[], ...overrides,
  }
}
