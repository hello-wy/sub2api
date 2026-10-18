<template><div class="relative h-64" role="img" aria-label="每日确认收入、总成本与利润趋势"><Line v-if="daily.length" :data="chartData" :options="options" /><p v-else class="py-12 text-center text-sm text-gray-500">{{ op('empty') }}</p></div></template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const op = (key: string) => t('admin.businessAnalytics.operating.' + key)
import { useMutationObserver } from '@vueuse/core'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { BusinessDay } from '@/api/admin/business'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const props = defineProps<{daily: BusinessDay[]}>()
const dark = ref(document.documentElement.classList.contains('dark'))
useMutationObserver(document.documentElement, () => { dark.value = document.documentElement.classList.contains('dark') }, {attributes: true, attributeFilter: ['class']})
const chartData = computed(() => ({ labels: props.daily.map(d => d.date.slice(5)), datasets: [
 {label: op('revenue'), data: props.daily.map(d => Number(d.revenue)), borderColor: dark.value ? '#60a5fa' : '#2563eb', pointRadius: props.daily.length > 31 ? 0 : 2, borderWidth: 2, tension: 0.15},
 {label: op('cost'), data: props.daily.map(d => Number(d.cost)), borderColor: dark.value ? '#fbbf24' : '#b45309', pointRadius: 0, borderWidth: 2, tension: 0.15},
 {label: op('estimate'), data: props.daily.map(d => d.profit === null ? null : Number(d.profit)), borderColor: dark.value ? '#34d399' : '#047857', pointRadius: 0, borderWidth: 2, tension: 0.15},
]}))
const options = computed<ChartOptions<'line'>>(() => ({responsive: true, maintainAspectRatio: false, animation: false, interaction: {intersect: false, mode: 'index'}, plugins: {legend: {position: 'bottom', labels: {usePointStyle: true, boxWidth: 7, boxHeight: 7, padding: 20, color: dark.value ? '#d1d5db' : '#4b5563'}}, tooltip: {callbacks: {label: item => item.dataset.label + '：' + (item.parsed.y === null ? op('pending') : new Intl.NumberFormat('zh-CN', {style:'currency',currency:'CNY'}).format(item.parsed.y))}}}, scales: {x: {grid: {display: false}, ticks: {maxTicksLimit: 12, color: dark.value ? '#9ca3af' : '#6b7280'}}, y: {grid: {color: dark.value ? '#ffffff0d' : '#0000000a'}, ticks: {color: dark.value ? '#9ca3af' : '#6b7280'}, title: {display: true, text: op('currency'), color: dark.value ? '#9ca3af' : '#6b7280'}}}}))
</script>
