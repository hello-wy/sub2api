<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.subscriptionPlanDistribution') }}</h3>
    <div v-if="!plans.length" class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('payment.admin.noData') }}
    </div>
    <div v-else class="space-y-3">
      <div v-for="plan in plans" :key="plan.plan_id" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-1">
        <div class="min-w-0">
          <div class="flex items-center justify-between gap-3">
            <span class="truncate text-sm font-medium text-gray-700 dark:text-gray-300" :title="planLabel(plan)">{{ planLabel(plan) }}</span>
            <span class="shrink-0 text-xs font-semibold text-gray-900 dark:text-white">{{ plan.count }}</span>
          </div>
          <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div class="h-full rounded-full bg-cyan-500" :style="{ width: `${(plan.count / maxCount) * 100}%` }" />
          </div>
        </div>
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.purchaseCount') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlanPurchaseStats } from '@/types/payment'

const { t } = useI18n()

const props = defineProps<{
  plans: SubscriptionPlanPurchaseStats[]
}>()

const maxCount = computed(() => Math.max(...props.plans.map(plan => plan.count), 1))
const planLabel = (plan: SubscriptionPlanPurchaseStats) => plan.plan_name || `${t('payment.admin.unknownSubscriptionPlan')} #${plan.plan_id}`
</script>
