<template>
  <AppLayout>
    <ScrollablePageLayout>
      <div class="mx-auto w-full max-w-[1440px] px-4 py-6 sm:px-6 lg:px-8">
        <header class="mb-7 flex flex-wrap items-end justify-between gap-5">
          <div><h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">经营分析</h1><p class="mt-2 text-sm text-gray-500 dark:text-gray-400">看清投入、消费收入，以及还需要提供的服务。</p></div>
          <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" />
        </header>
        <BusinessAnalyticsPanel :start-date="startDate" :end-date="endDate" @dates="setDates" />
      </div>
    </ScrollablePageLayout>
  </AppLayout>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ScrollablePageLayout from '@/components/layout/ScrollablePageLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import BusinessAnalyticsPanel from '@/components/admin/usage/BusinessAnalyticsPanel.vue'
const startDate = ref('')
const endDate = ref('')
function setDates(start: string, end: string) { if (!startDate.value && !endDate.value) { startDate.value = start; endDate.value = end } }
</script>
