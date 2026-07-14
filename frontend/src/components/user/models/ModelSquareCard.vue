<template>
  <article
    data-test="model-card-shell"
    class="min-w-0 rounded-xl border border-gray-200 bg-white px-4 py-4 dark:border-dark-700 dark:bg-dark-800"
    :class="viewMode === 'list' && 'lg:px-5'"
  >
    <div data-test="card-content" :class="viewMode === 'list' ? 'flex min-w-0 flex-col gap-4 lg:grid lg:grid-cols-[minmax(0,11fr)_minmax(0,9fr)] lg:items-start lg:gap-0' : ''">
      <div data-test="card-identity" :class="viewMode === 'list' ? 'flex min-w-0 flex-1 items-start justify-between gap-4 lg:pr-4 xl:pr-6' : 'flex min-w-0 items-start justify-between gap-3'">
        <div class="flex min-w-0 items-start gap-3">
          <span class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-gray-50 ring-1 ring-black/5 dark:bg-dark-700 dark:ring-white/10">
            <PlatformIcon :platform="card.platform" size="lg" :class="platformIconClass(card.platform)" />
          </span>
          <div class="min-w-0">
            <h2 class="truncate text-base font-semibold leading-5 text-gray-900 dark:text-gray-100" :title="card.model">{{ card.model }}</h2>
            <div class="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <span class="truncate text-xs text-gray-400">{{ card.platform }}</span>
              <span class="rounded bg-violet-50 px-1.5 py-0.5 text-[10px] font-semibold leading-4 text-violet-700 dark:bg-violet-500/10 dark:text-violet-300">{{ billingLabel }}</span>
            </div>
            <p v-if="viewMode === 'list'" class="mt-1 truncate text-xs text-gray-400">{{ card.channel }}</p>
          </div>
        </div>

        <div v-if="activeGroup" class="shrink-0 text-right">
          <div class="text-[10px] text-gray-400">{{ t('modelSquare.currentGroup') }}</div>
          <div class="mt-0.5 max-w-[92px] truncate text-sm font-semibold text-gray-700 dark:text-gray-200" :title="activeGroup.name">{{ activeGroup.name }}</div>
          <div class="numeric text-xs font-medium text-primary-600 dark:text-primary-300">{{ formatPrice(effectiveRate) }}×</div>
        </div>
      </div>

      <div
        v-if="priceLines.length"
        data-test="price-grid"
        class="mt-4 grid min-w-0 grid-cols-2 gap-x-3 gap-y-3 border-t border-gray-100 pt-3 dark:border-dark-700"
        :class="viewMode === 'list' && 'sm:grid-cols-4 lg:mt-0 lg:grid-cols-2 lg:gap-x-[clamp(0.75rem,2vw,2rem)] lg:border-l lg:border-t-0 lg:pl-5 lg:pt-0 xl:grid-cols-4'"
      >
        <div v-for="line in priceLines" :key="line.key" class="min-w-0">
          <div class="truncate text-[10px] text-gray-400">{{ line.label }}</div>
          <div data-test="price-value" class="numeric mt-1 whitespace-nowrap text-sm font-medium text-gray-800 dark:text-gray-200" :title="`$${line.value} ${line.unit}`">
            ${{ line.value }}
          </div>
          <div class="mt-0.5 truncate text-[10px] text-gray-400">{{ line.unit }}</div>
        </div>
        <div v-if="intervals.length" class="col-span-full text-[10px] text-gray-400" :title="t('modelSquare.intervalHint')">{{ t('modelSquare.intervalPricing', { count: intervals.length }) }}</div>
      </div>
      <div v-else data-test="no-pricing" class="mt-4 border-t border-gray-100 pt-3 text-sm text-gray-400 dark:border-dark-700" :class="viewMode === 'list' && 'lg:mt-0 lg:border-l lg:border-t-0 lg:pl-5 lg:pt-2'">{{ t('modelSquare.noPricing') }}</div>
    </div>

    <template v-if="monitor">
      <div data-test="monitor-summary" class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700">
        <div class="flex min-w-0 flex-wrap items-end gap-x-5 gap-y-2">
          <div>
            <div class="text-[10px] text-gray-400">{{ t('modelSquare.monitor.status') }}</div>
            <span class="mt-1 inline-flex rounded-full px-2 py-0.5 text-xs font-semibold" :class="statusBadgeClass(monitor.primary_status)">{{ statusLabel(monitor.primary_status) }}</span>
          </div>
          <div v-for="metric in monitorMetrics" :key="metric.label" class="min-w-0">
            <div class="text-[10px] text-gray-400">{{ metric.label }}</div>
            <div class="numeric mt-1 whitespace-nowrap text-sm font-medium text-gray-700 dark:text-gray-200">
              {{ metric.value }}<span v-if="metric.unit" class="ml-1 text-[10px] font-normal text-gray-400">{{ metric.unit }}</span>
            </div>
          </div>
        </div>
      </div>
      <MonitorTimeline
        class="min-w-0"
        :buckets="monitor.timeline"
        :countdown-seconds="0"
        :show-countdown="false"
      />
    </template>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserAvailableGroup, UserPricingInterval, UserSupportedModelPricing } from '@/api/channels'
import type { UserMonitorView } from '@/api/channelMonitor'
import type { BillingMode } from '@/constants/channel'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import MonitorTimeline from '@/components/user/monitor/MonitorTimeline.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { equivalentPrice, formatPrice, normalizeRechargeMultiplier } from '@/utils/model-pricing'
import { platformIconClass } from '@/utils/platformColors'

export interface ModelSquareCardData {
  key: string
  model: string
  platform: string
  channel: string
  groups: UserAvailableGroup[]
  pricing: UserSupportedModelPricing | null
  billingMode: BillingMode
  searchText: string
}

const props = withDefaults(defineProps<{
  card: ModelSquareCardData
  activeGroup?: UserAvailableGroup
  effectiveRate: number
  rechargeMultiplier: number
  monitor?: UserMonitorView
  viewMode?: 'grid' | 'list'
}>(), {
  viewMode: 'grid',
})
const { t } = useI18n()
const { statusLabel, statusBadgeClass, formatLatency, formatPercent } = useChannelMonitorFormat()
const billingLabel = computed(() => t(props.card.billingMode === 'token' ? 'modelSquare.billing.usageBased' : 'modelSquare.billing.perImage'))
const intervals = computed<UserPricingInterval[]>(() => props.card.pricing?.intervals ?? [])

function directEquivalentPrice(value: number | null | undefined): number | null {
  if (value == null) return null
  return value * props.effectiveRate / normalizeRechargeMultiplier(props.rechargeMultiplier)
}
function makeLine(key: string, label: string, value: number | null | undefined, token: boolean) {
  if (value == null || !Number.isFinite(Number(value)) || Number(value) === 0) return null
  const price = token
    ? equivalentPrice(value, props.effectiveRate, props.rechargeMultiplier)
    : directEquivalentPrice(value)
  return price == null ? null : {
    key,
    label,
    value: formatPrice(price),
    unit: token ? t('modelSquare.units.perMillion') : t('modelSquare.units.perRequest'),
  }
}
const monitorMetrics = computed(() => {
  if (!props.monitor) return []
  return [
    { label: t('modelSquare.monitor.throughput'), value: formatThroughput(props.monitor.primary_throughput_tps), unit: 'TPS' },
    { label: t('modelSquare.monitor.latency'), value: formatLatency(props.monitor.primary_latency_ms), unit: 'ms' },
    { label: t('monitorCommon.endpointPing'), value: formatLatency(props.monitor.primary_ping_latency_ms), unit: 'ms' },
    { label: t('monitorCommon.availabilityPrefix'), value: formatPercent(props.monitor.availability_7d), unit: '' },
  ]
})
const priceLines = computed(() => {
  const pricing = props.card.pricing
  if (!pricing) return []
  if (pricing.billing_mode !== 'token') {
    const line = makeLine('request', t('modelSquare.price.perRequest'), pricing.per_request_price, false)
    return line ? [line] : []
  }
  return [
    makeLine('input', t('modelSquare.price.input'), pricing.input_price, true),
    makeLine('output', t('modelSquare.price.output'), pricing.output_price, true),
    makeLine('cacheWrite', t('modelSquare.price.cacheWrite'), pricing.cache_write_price, true),
    makeLine('cacheRead', t('modelSquare.price.cacheRead'), pricing.cache_read_price, true),
    makeLine('imageOutput', t('modelSquare.price.imageOutput'), pricing.image_output_price, true),
  ].filter((line): line is NonNullable<typeof line> => line !== null)
})
function formatThroughput(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return formatPrice(value)
}
</script>

<style scoped>
.numeric { font-variant-numeric: tabular-nums; }
</style>
