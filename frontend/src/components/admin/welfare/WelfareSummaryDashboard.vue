<template>
  <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
    <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
      <div class="text-sm text-gray-500 dark:text-dark-300">{{ t('admin.welfare.dashboard.totalRewards') }}</div>
      <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
        {{ summary.total_count }}
      </div>
    </div>
    <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800 sm:col-span-2">
      <div class="text-sm text-gray-500 dark:text-dark-300">{{ t('admin.welfare.dashboard.totalAmount') }}</div>
      <div class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
        ${{ summary.total_amount.toFixed(2) }}
      </div>
      <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
        {{ t('admin.welfare.dashboard.breakdown', {
          checkin: summary.checkin_amount.toFixed(2),
          leaderboard: summary.leaderboard_amount.toFixed(2),
          lottery: summary.lottery_amount.toFixed(2)
        }) }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { WelfareSummary } from '@/api/admin/welfare'

defineProps<{
  summary: WelfareSummary
}>()

const { t } = useI18n()
</script>
