<template>
  <div class="card p-4">
    <div class="mb-4 flex items-baseline justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('payment.admin.groupRevenueEfficiency') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('payment.admin.groupRevenueEfficiencyDescription') }}
        </p>
      </div>
    </div>

    <div v-if="!groups.length" class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('payment.admin.noData') }}
    </div>
    <div v-else class="-mx-4 overflow-x-auto">
      <table class="w-full min-w-[960px] text-sm">
        <thead class="border-y border-gray-100 bg-gray-50/70 text-xs text-gray-500 dark:border-dark-700 dark:bg-dark-800/50 dark:text-gray-400">
          <tr>
            <th scope="col" class="px-4 py-3 text-left font-medium">{{ t('payment.admin.groupName') }}</th>
            <th scope="col" class="px-4 py-3 text-right font-medium">{{ t('payment.admin.groupMultiplier') }}</th>
            <th scope="col" class="px-4 py-3 text-right font-medium">{{ t('payment.admin.groupRevenue') }}</th>
            <th scope="col" class="px-4 py-3 text-right font-medium">{{ t('payment.admin.expectedQuota') }}</th>
            <th scope="col" class="px-4 py-3 text-right font-medium">{{ t('payment.admin.userDeductedQuota') }}</th>
            <th scope="col" class="px-4 py-3 text-right font-medium">{{ t('payment.admin.groupBaseUsage') }}</th>
            <th scope="col" class="px-4 py-3 text-right font-medium">{{ t('payment.admin.unitRevenue') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-for="group in groups" :key="group.group_id" class="hover:bg-gray-50/70 dark:hover:bg-dark-800/40">
            <td class="max-w-64 px-4 py-3 font-medium text-gray-700 dark:text-gray-300">
              <span class="block truncate" :title="groupLabel(group)">{{ groupLabel(group) }}</span>
            </td>
            <td class="whitespace-nowrap px-4 py-3 text-right text-gray-700 dark:text-gray-300">{{ formatMultiplier(group.rate_multiplier) }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-right font-medium text-gray-900 dark:text-white">{{ formatMoney(group.revenue) }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-right text-gray-700 dark:text-gray-300">{{ group.expected_quota === null ? '-' : formatQuota(group.expected_quota) }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-right text-gray-700 dark:text-gray-300">{{ formatQuota(group.user_usage) }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-right text-gray-700 dark:text-gray-300">{{ formatQuota(group.base_usage) }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-right font-semibold text-gray-900 dark:text-white">
              {{ group.unit_revenue === null ? t('payment.admin.noUsage') : formatMoney(group.unit_revenue) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { GroupRevenueEfficiencyStats } from '@/types/payment'

const { t } = useI18n()

const props = defineProps<{
  groups: GroupRevenueEfficiencyStats[]
  currency: string
}>()

function groupLabel(group: GroupRevenueEfficiencyStats): string {
  return group.group_name || `${t('payment.admin.unknownGroup')} #${group.group_id}`
}

function formatMoney(amount: number): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: props.currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(amount)
}

function formatQuota(amount: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 4 }).format(amount)
}

function formatMultiplier(multiplier: number): string {
  if (!Number.isFinite(multiplier)) return '-'
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 4 }).format(multiplier)}x`
}
</script>
