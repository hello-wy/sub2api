<template>
  <div class="analytics-charts">
    <div class="analytics-chart analytics-chart--trend">
      <div class="analytics-chart__head">
        <strong>利润走势</strong>
        <small>人民币</small>
      </div>
      <div class="analytics-chart__canvas">
        <Line v-if="trendData" :data="trendData" :options="trendOptions" />
        <div v-else class="analytics-chart__empty">当前区间没有可绘制的每日数据</div>
      </div>
    </div>

    <div class="analytics-chart analytics-chart--groups">
      <div class="analytics-chart__head">
        <strong>分组贡献</strong>
        <small>收入 Top 8</small>
      </div>
      <div class="analytics-chart__canvas">
        <Bar v-if="groupData" :data="groupData" :options="groupOptions" />
        <div v-else class="analytics-chart__empty">当前区间没有可绘制的分组数据</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip,
  type ChartData,
  type ChartOptions,
} from 'chart.js'
import { Bar, Line } from 'vue-chartjs'
import type { BusinessDailyAnalytics, BusinessGroupAnalytics } from '@/api/admin/dashboard'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Tooltip, Legend, Filler)

const props = defineProps<{
  daily: BusinessDailyAnalytics[]
  groups: BusinessGroupAnalytics[]
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

const currency = (value: number) => `¥${Number(value || 0).toFixed(2)}`
const shortDate = (value: string) => value.length >= 10 ? value.slice(5) : value
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#d1d5db' : '#4b5563',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  revenue: '#3b82f6',
  profit: '#10b981',
  cost: '#f59e0b',
  loss: '#ef4444',
}))

const trendData = computed<ChartData<'line'> | null>(() => {
  if (!props.daily.length) return null
  return {
    labels: props.daily.map((day) => shortDate(day.date)),
    datasets: [
      {
        label: '折算收入',
        data: props.daily.map((day) => day.usage_revenue_cny),
        borderColor: chartColors.value.revenue,
        backgroundColor: `${chartColors.value.revenue}18`,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3,
        tension: 0.35,
        fill: true,
      },
      {
        label: '总成本',
        data: props.daily.map((day) => day.api_key_usage_cost_cny + day.welfare_cost_cny + day.account_cost_cny),
        borderColor: chartColors.value.cost,
        backgroundColor: 'transparent',
        borderWidth: 1.5,
        borderDash: [5, 4],
        pointRadius: 0,
        pointHoverRadius: 3,
        tension: 0.35,
      },
      {
        label: '经营利润',
        data: props.daily.map((day) => day.operating_profit_cny),
        borderColor: chartColors.value.profit,
        backgroundColor: 'transparent',
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3,
        tension: 0.35,
      },
    ],
  }
})

const trendOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index', intersect: false },
  animation: { duration: 520, easing: 'easeOutQuart' },
  plugins: {
    legend: { position: 'top', align: 'start', labels: { color: chartColors.value.text, boxWidth: 8, boxHeight: 8, usePointStyle: true, pointStyle: 'circle', padding: 16, font: { size: 11 } } },
    tooltip: { callbacks: { label: (context) => `${context.dataset.label}: ${currency(context.parsed.y ?? 0)}` } },
  },
  scales: {
    x: { border: { display: false }, grid: { display: false }, ticks: { color: chartColors.value.text, maxRotation: 0, autoSkipPadding: 18, font: { size: 10 } } },
    y: { beginAtZero: true, border: { display: false }, grid: { color: chartColors.value.grid }, ticks: { color: chartColors.value.text, font: { size: 10 }, callback: (value) => `¥${value}` } },
  },
}))

const sortedGroups = computed(() => [...props.groups].sort((left, right) => right.usage_revenue_cny - left.usage_revenue_cny).slice(0, 8))

const groupData = computed<ChartData<'bar'> | null>(() => {
  if (!sortedGroups.value.length) return null
  return {
    labels: sortedGroups.value.map((group) => group.group_name || `分组 #${group.group_id}`),
    datasets: [
      { label: '收入', data: sortedGroups.value.map((group) => group.usage_revenue_cny), backgroundColor: `${chartColors.value.revenue}cc`, borderRadius: 3, barPercentage: 0.7 },
      { label: '总成本', data: sortedGroups.value.map((group) => group.api_key_usage_cost_cny + group.allocated_welfare_cost_cny + group.allocated_account_cost_cny), backgroundColor: `${chartColors.value.cost}c0`, borderRadius: 3, barPercentage: 0.7 },
      { label: '利润', data: sortedGroups.value.map((group) => group.operating_profit_cny), backgroundColor: sortedGroups.value.map((group) => group.operating_profit_cny >= 0 ? chartColors.value.profit : chartColors.value.loss), borderRadius: 3, barPercentage: 0.7 },
    ],
  }
})

const groupOptions = computed<ChartOptions<'bar'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  indexAxis: 'y',
  interaction: { mode: 'index', intersect: false },
  animation: { duration: 520, easing: 'easeOutQuart' },
  plugins: {
    legend: { position: 'top', align: 'start', labels: { color: chartColors.value.text, boxWidth: 8, boxHeight: 8, usePointStyle: true, pointStyle: 'circle', padding: 16, font: { size: 11 } } },
    tooltip: { callbacks: { label: (context) => `${context.dataset.label}: ${currency(context.parsed.x ?? 0)}` } },
  },
  scales: {
    x: { beginAtZero: true, border: { display: false }, grid: { color: chartColors.value.grid }, ticks: { color: chartColors.value.text, font: { size: 10 }, callback: (value) => `¥${value}` } },
    y: { grid: { display: false }, border: { display: false }, ticks: { color: chartColors.value.text, font: { size: 10 } } },
  },
}))
</script>

<style scoped>
.analytics-charts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.5rem; min-height: 21rem; }
.analytics-chart { min-width: 0; padding: 1rem; border: 1px solid rgb(229 231 235); border-radius: .75rem; background: rgb(255 255 255); }
.analytics-chart__head { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: .5rem; }
.analytics-chart__head strong { color: rgb(17 24 39); font-size: .875rem; font-weight: 600; }
.analytics-chart__head small { color: rgb(156 163 175); font-size: .75rem; }
.analytics-chart__canvas { position: relative; height: 17rem; }
.analytics-chart__empty { display: grid; height: 100%; place-items: center; color: rgb(107 114 128); font-size: .875rem; }
:global(.dark) .analytics-chart { border-color: rgb(55 65 81); background: rgb(17 24 39); }
:global(.dark) .analytics-chart__head strong { color: rgb(243 244 246); }
@media (max-width: 900px) {
  .analytics-charts { grid-template-columns: 1fr; }
}
</style>
