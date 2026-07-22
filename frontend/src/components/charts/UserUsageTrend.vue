<template>
  <div class="user-usage-trend">
    <div v-if="loading" class="user-usage-trend__state">
      <LoadingSpinner />
    </div>
    <div v-else-if="chartData" class="user-usage-trend__canvas">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div v-else class="user-usage-trend__state text-sm text-gray-500 dark:text-gray-400">
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
  Tooltip,
  Legend,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { UserUsageTrendPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const props = defineProps<{
  trendData: UserUsageTrendPoint[]
  loading?: boolean
}>()

const { t } = useI18n()
const isDarkMode = ref(document.documentElement.classList.contains('dark'))
let themeObserver: MutationObserver | null = null

const palette = [
  '#347cff',
  '#10a981',
  '#e59a0a',
  '#e05252',
  '#805ad5',
  '#d84f91',
  '#159aa5',
  '#e56f2d',
  '#596dd9',
  '#78a82b',
  '#1697c4',
  '#9b55cc'
]

onMounted(() => {
  themeObserver = new MutationObserver(() => {
    isDarkMode.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})

onUnmounted(() => themeObserver?.disconnect())

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#cbd5e1' : '#5f6f89',
  grid: isDarkMode.value ? 'rgba(148, 163, 184, 0.14)' : 'rgba(214, 224, 238, 0.72)'
}))

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  const users = new Map<number, { label: string; values: Map<string, number> }>()
  const dates = new Set<string>()

  props.trendData.forEach((point) => {
    dates.add(point.date)
    const label = point.email?.trim() || point.username?.trim() || `用户 #${point.user_id}`
    const user = users.get(point.user_id) || { label, values: new Map<string, number>() }
    user.values.set(point.date, point.tokens)
    users.set(point.user_id, user)
  })

  const sortedDates = Array.from(dates).sort()
  const datasets = Array.from(users.values()).map((user, index) => {
    const color = palette[index % palette.length]
    return {
      label: user.label,
      data: sortedDates.map((date) => user.values.get(date) || 0),
      borderColor: color,
      backgroundColor: color,
      borderWidth: 1.8,
      pointRadius: 0,
      pointHoverRadius: 4,
      pointHitRadius: 10,
      tension: 0.32,
      fill: false
    }
  })

  return { labels: sortedDates, datasets }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  animation: false as const,
  plugins: {
    legend: {
      position: 'top' as const,
      align: 'start' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle' as const,
        boxWidth: 7,
        boxHeight: 7,
        padding: 14,
        font: { size: 11 }
      }
    },
    tooltip: {
      itemSort: (a: TooltipItem<'line'>, b: TooltipItem<'line'>) => Number(b.parsed.y ?? 0) - Number(a.parsed.y ?? 0),
      callbacks: {
        label: (context: TooltipItem<'line'>) => `${context.dataset.label}: ${formatTokens(Number(context.parsed.y ?? 0))}`
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: {
        color: chartColors.value.text,
        maxRotation: 0,
        autoSkipPadding: 18,
        font: { size: 10 }
      }
    },
    y: {
      beginAtZero: true,
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        font: { size: 10 },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

function formatTokens(value: number): string {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`
  return value.toLocaleString()
}
</script>

<style scoped>
.user-usage-trend {
  margin-top: 14px;
}

.user-usage-trend__canvas,
.user-usage-trend__state {
  height: 19rem;
}

.user-usage-trend__state {
  display: grid;
  place-items: center;
}

@media (max-width: 760px) {
  .user-usage-trend__canvas,
  .user-usage-trend__state {
    height: 22rem;
  }
}

</style>
