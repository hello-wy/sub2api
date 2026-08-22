<template>
  <div class="token-usage-trend" :class="embedded ? 'token-usage-trend--embedded' : 'card p-4'">
    <h3 v-if="!embedded" class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.dashboard.tokenUsageTrend') }}
    </h3>
    <div v-if="loading" class="token-usage-trend__state flex items-center justify-center" :class="embedded ? 'token-usage-trend__state--embedded' : 'h-48'">
      <LoadingSpinner />
    </div>
    <div v-else-if="trendData.length > 0 && chartData" class="token-usage-trend__canvas" :class="embedded ? 'token-usage-trend__canvas--embedded' : 'h-48'">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="token-usage-trend__state flex items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      :class="embedded ? 'token-usage-trend__state--embedded' : 'h-48'"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'
import type { DailyPaymentStats } from '@/types/payment'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
  embedded?: boolean
  totalOnly?: boolean
  rechargeSeries?: DailyPaymentStats[]
  rechargeCurrency?: string
}>()

const isDarkMode = ref(document.documentElement.classList.contains('dark'))
let themeObserver: MutationObserver | null = null

onMounted(() => {
  themeObserver = new MutationObserver(() => {
    isDarkMode.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})

onUnmounted(() => themeObserver?.disconnect())

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  input: '#3b82f6',
  output: '#10b981',
  cacheCreation: '#f59e0b',
  cacheRead: '#06b6d4',
  cacheHitRate: '#8b5cf6',
  recharge: '#ef6c4d'
}))

const rechargeCurrency = computed(() => props.rechargeCurrency?.trim().toUpperCase() || '')
const rechargeByDate = computed(() => {
  const values = new Map<string, number>()
  for (const day of props.rechargeSeries || []) {
    const amount = rechargeCurrency.value ? day.amount[rechargeCurrency.value] : 0
    values.set(String(day.date).slice(0, 10), Number.isFinite(amount) ? amount : 0)
  }
  return values
})
const hasRechargeSeries = computed(() => props.totalOnly && rechargeCurrency.value !== '' && (props.rechargeSeries?.length || 0) > 0)
const rechargeLabel = computed(() => `Recharge (${rechargeCurrency.value})`)

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  if (props.totalOnly) {
    return {
      labels: props.trendData.map((d) => d.date),
      datasets: [
        {
          label: 'Total Tokens',
          data: props.trendData.map((d) => d.total_tokens),
          borderColor: chartColors.value.input,
          backgroundColor: `${chartColors.value.input}20`,
          fill: true,
          tension: 0.38,
          borderWidth: 2,
          pointRadius: 0,
          pointHoverRadius: 3
        },
        ...(hasRechargeSeries.value ? [{
          label: rechargeLabel.value,
          data: props.trendData.map((d) => rechargeByDate.value.get(String(d.date).slice(0, 10)) || 0),
          borderColor: chartColors.value.recharge,
          backgroundColor: `${chartColors.value.recharge}20`,
          fill: false,
          tension: 0.38,
          borderWidth: 2,
          pointRadius: 0,
          pointHoverRadius: 3,
          yAxisID: 'yRecharge'
        }] : [])
      ]
    }
  }

  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: 'Input',
        data: props.trendData.map((d) => d.input_tokens),
        borderColor: chartColors.value.input,
        backgroundColor: `${chartColors.value.input}20`,
        fill: true,
        tension: 0.38,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3
      },
      {
        label: 'Output',
        data: props.trendData.map((d) => d.output_tokens),
        borderColor: chartColors.value.output,
        backgroundColor: `${chartColors.value.output}20`,
        fill: true,
        tension: 0.38,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3
      },
      {
        label: 'Cache Creation',
        data: props.trendData.map((d) => d.cache_creation_tokens),
        borderColor: chartColors.value.cacheCreation,
        backgroundColor: `${chartColors.value.cacheCreation}20`,
        fill: true,
        tension: 0.38,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3
      },
      {
        label: 'Cache Read',
        data: props.trendData.map((d) => d.cache_read_tokens),
        borderColor: chartColors.value.cacheRead,
        backgroundColor: `${chartColors.value.cacheRead}20`,
        fill: true,
        tension: 0.38,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3
      },
      {
        label: 'Cache Hit Rate',
        data: props.trendData.map((d) => {
          const totalPromptTokens = d.input_tokens + d.cache_read_tokens + d.cache_creation_tokens
          return totalPromptTokens > 0 ? (d.cache_read_tokens / totalPromptTokens) * 100 : 0
        }),
        borderColor: chartColors.value.cacheHitRate,
        backgroundColor: `${chartColors.value.cacheHitRate}20`,
        borderDash: [5, 5],
        fill: false,
        tension: 0.3,
        borderWidth: 1.5,
        pointRadius: 0,
        pointHoverRadius: 3,
        yAxisID: 'yPercent'
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  animation: {
    duration: 520,
    easing: 'easeOutQuart' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          if (context.dataset.yAxisID === 'yPercent') {
            return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
          }
          if (context.dataset.yAxisID === 'yRecharge') {
            return `${context.dataset.label}: ${formatRecharge(context.raw)}`
          }
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        },
        footer: (tooltipItems: any) => {
          if (props.totalOnly) return ''
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && props.trendData[dataIndex]) {
            const data = props.trendData[dataIndex]
            return `Actual: $${formatCost(data.actual_cost)} | Standard: $${formatCost(data.cost)}`
          }
          return ''
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    },
    ...(hasRechargeSeries.value ? {
      yRecharge: {
        position: 'right' as const,
        min: 0,
        grid: {
          drawOnChartArea: false
        },
        ticks: {
          color: chartColors.value.recharge,
          font: {
            size: 10
          },
          callback: (value: string | number) => formatRecharge(Number(value))
        }
      }
    } : {}),
    ...(!props.totalOnly ? {
      yPercent: {
        position: 'right' as const,
        min: 0,
        max: 100,
        grid: {
          drawOnChartArea: false
        },
        ticks: {
          color: chartColors.value.cacheHitRate,
          font: {
            size: 10
          },
          callback: (value: string | number) => `${value}%`
        }
      }
    } : {})
  }
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}

const formatRecharge = (value: number): string => `${rechargeCurrency.value} ${formatCost(value)}`
</script>

<style scoped>
.token-usage-trend--embedded {
  margin-top: 12px;
}

.token-usage-trend__canvas--embedded,
.token-usage-trend__state--embedded {
  height: 12.5rem;
}
</style>
