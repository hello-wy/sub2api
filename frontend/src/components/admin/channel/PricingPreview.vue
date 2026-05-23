<template>
  <div v-if="rows.length > 0" class="mt-3 rounded-lg border border-primary-200 bg-primary-50/70 p-3 dark:border-primary-900/50 dark:bg-primary-950/20">
    <div class="mb-2 flex items-center justify-between gap-2">
      <span class="text-xs font-medium text-primary-700 dark:text-primary-300">
        {{ t('admin.channels.form.pricePreview', '价格计算预览') }}
      </span>
      <span class="text-[11px] text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.form.rawPriceSaved', '仅保存原始价格') }}
      </span>
    </div>
    <div class="grid gap-2 md:grid-cols-2">
      <div v-for="row in rows" :key="row.groupId" class="rounded-md bg-white/70 p-2 text-xs dark:bg-dark-900/50">
        <div class="flex items-center justify-between">
          <span class="font-medium text-gray-700 dark:text-gray-200">{{ row.name }}</span>
          <span class="rounded-full bg-primary-100 px-1.5 py-0.5 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">{{ row.rate }}x</span>
        </div>
        <div class="mt-1 grid grid-cols-2 gap-2 text-gray-500 dark:text-gray-400">
          <span>{{ t('admin.channels.form.inputPrice', '输入') }}: {{ row.input }}</span>
          <span>{{ t('admin.channels.form.outputPrice', '输出') }}: {{ row.output }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminGroup } from '@/types'
import { applyRateMultiplier, formatPrice } from '@/utils/model-pricing'
import type { PricingFormEntry } from './types'

const props = defineProps<{
  entry: PricingFormEntry
  groups?: readonly AdminGroup[]
}>()

const { t } = useI18n()

const rows = computed(() => {
  if (props.entry.billing_mode !== 'token') return []
  return (props.groups || []).map(group => ({
    groupId: group.id,
    name: group.name,
    rate: group.rate_multiplier,
    input: previewPrice(props.entry.input_price, group.rate_multiplier),
    output: previewPrice(props.entry.output_price, group.rate_multiplier),
  }))
})

function previewPrice(price: number | string | null, rate: number): string {
  const calculated = applyRateMultiplier(price, rate)
  return calculated === null ? '-' : `$${formatPrice(calculated)}`
}
</script>
