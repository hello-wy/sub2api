<template>
  <AppLayout>
    <ScrollablePageLayout>
      <div class="business-analytics-page mx-auto w-full max-w-[1680px] space-y-6">
        <header class="business-analytics-hero">
          <BusinessPanelNavigation v-model="activeTab" />

          <div class="business-analytics-toolbar">
            <div class="business-analytics-actions" aria-label="经营账操作">
              <button type="button" class="btn btn-primary inline-flex items-center gap-1.5" @click="openPanelRecord('expense')"><Icon name="dollar" size="sm" />录入费用</button>
              <button type="button" class="btn btn-secondary inline-flex items-center gap-1.5" @click="openPanelRecord('receipt')"><Icon name="plus" size="sm" />登记收款</button>
              <button type="button" class="btn btn-secondary inline-flex items-center gap-1.5" @click="openPanelRecord('purchase')"><Icon name="database" size="sm" />登记采购</button>
              <button type="button" class="btn btn-secondary business-analytics-refresh" :disabled="panelRefreshing" aria-label="刷新经营账" title="刷新经营账" @click="refreshPanel"><Icon name="refresh" size="sm" :class="panelRefreshing ? 'animate-spin' : ''" /></button>
            </div>

            <div class="business-analytics-date-control">
              <span class="business-analytics-hero__label">统计周期</span>
              <div class="business-analytics-date-picker">
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="handleDateRangeChange"
                />
              </div>
            </div>
          </div>
        </header>

        <BusinessAnalyticsPanel ref="panelRef" v-model:active-tab="activeTab" :start-date="startDate" :end-date="endDate" :show-header="false" :show-tabs="false" />
      </div>
    </ScrollablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ScrollablePageLayout from '@/components/layout/ScrollablePageLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Icon from '@/components/icons/Icon.vue'
import BusinessAnalyticsPanel from '@/components/admin/usage/BusinessAnalyticsPanel.vue'
import BusinessPanelNavigation, { type BusinessTab } from '@/components/admin/usage/BusinessPanelNavigation.vue'

const activeTab = ref<BusinessTab>('overview')
const panelRef = ref<InstanceType<typeof BusinessAnalyticsPanel> | null>(null)
const panelRefreshing = ref(false)

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const today = new Date()
const monthStart = new Date(today.getFullYear(), today.getMonth(), 1)
const startDate = ref(formatLocalDate(monthStart))
const endDate = ref(formatLocalDate(today))

const handleDateRangeChange = (range: { startDate: string; endDate: string }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
}

const openPanelRecord = (type: string) => panelRef.value?.openRecord(type)
const refreshPanel = async () => {
  if (panelRefreshing.value) return
  panelRefreshing.value = true
  try {
    await panelRef.value?.refresh()
  } finally {
    panelRefreshing.value = false
  }
}
</script>

<style scoped>
.business-analytics-hero {
  @apply flex flex-wrap items-center justify-between gap-x-6 gap-y-5 py-1;
}

.business-analytics-toolbar {
  @apply ml-auto flex max-w-full flex-wrap items-center justify-end gap-x-4 gap-y-3;
}

.business-analytics-date-control {
  @apply ml-auto flex max-w-full items-center gap-3 sm:border-l sm:border-gray-200 sm:pl-4 sm:dark:border-dark-700;
}

.business-analytics-actions {
  @apply flex flex-wrap items-center gap-2;
}

.business-analytics-actions .btn {
  @apply h-9 rounded-lg px-3 text-xs font-medium shadow-none;
}

.business-analytics-actions .business-analytics-refresh {
  @apply inline-flex w-9 items-center justify-center p-0;
}

.business-analytics-date-picker {
  @apply relative z-30 max-w-full;
}

.business-analytics-date-picker :deep(.date-picker-trigger) {
  @apply min-h-9 min-w-0 justify-between rounded-lg py-1.5 text-xs shadow-none;
}

.business-analytics-date-picker :deep(.date-picker-dropdown) {
  left: auto;
  right: 0;
  min-width: min(320px, calc(100vw - 2rem));
  max-width: calc(100vw - 2rem);
}

.business-analytics-hero__label {
  @apply shrink-0 text-xs text-gray-500 dark:text-gray-400;
}

.business-analytics-date-picker :deep(.date-picker-value) {
  @apply text-xs;
}

@media (max-width: 639px) {
  .business-analytics-toolbar { @apply w-full justify-start; }
  .business-analytics-date-control { @apply w-full justify-between; }
}
</style>
