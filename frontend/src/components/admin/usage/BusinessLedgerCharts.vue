<template><div class="h-[245px]"><Line v-if="daily.length" :data="data" :options="options" /><p v-else class="py-12 text-center text-sm text-gray-500">当前区间没有收入或成本记录</p></div></template>
<script setup lang="ts">
import { computed } from 'vue'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { BusinessBreakdown } from '@/api/admin/business'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const props = defineProps<{ daily: BusinessBreakdown[]; view?: string }>()
const data = computed(() => ({ labels: props.daily.map(d => d.name), datasets: props.view === 'cash' ? [
  { label: '现金收款（人民币）', data: props.daily.map(d => Number(d.cash_in_cny || '0')), borderColor: '#2563eb', pointRadius: 1 },
  { label: '现金付款（人民币）', data: props.daily.map(d => Number(d.cash_out_cny || '0')), borderColor: '#d97706', pointRadius: 1 },
  { label: '现金退款（人民币）', data: props.daily.map(d => Number(d.cash_refund_cny || '0')), borderColor: '#dc2626', pointRadius: 1 },
  { label: '现金净流入（人民币）', data: props.daily.map(d => Number(d.cash_net_cny || '0')), borderColor: '#059669', pointRadius: 1 },
] : [
  { label: '确认收入（人民币）', data: props.daily.map(d => Number(d.revenue_cny)), borderColor: '#2563eb', pointRadius: 1 },
  { label: '已入账成本（人民币）', data: props.daily.map(d => Number(d.cost_cny)), borderColor: '#d97706', pointRadius: 1 },
  { label: '已知收支差额（人民币）', data: props.daily.map(d => Number(d.profit_cny)), borderColor: '#059669', pointRadius: 1 },
] }))
const options: ChartOptions<'line'> = { responsive: true, maintainAspectRatio: false, interaction: { intersect: false, mode: 'index' }, plugins: { legend: { position: 'bottom' } }, scales: { y: { title: { display: true, text: 'CNY' } } } }
</script>
