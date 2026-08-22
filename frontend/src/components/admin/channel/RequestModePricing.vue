<template>
  <div>
    <label class="mt-3 block text-xs font-medium text-gray-500 dark:text-gray-400">
      {{ mode === 'image' ? t('admin.channels.form.defaultImagePrice') : t('admin.channels.form.defaultPerRequestPrice') }}
      <span class="ml-1 font-normal text-gray-400">$</span>
    </label>
    <div class="mt-1 w-48">
      <input
        :value="entry.per_request_price"
        type="number"
        step="any"
        min="0"
        class="input text-sm"
        :placeholder="t('admin.channels.form.pricePlaceholder')"
        @input="emitField(($event.target as HTMLInputElement).value)"
      />
    </div>

    <div class="mt-3 flex items-center justify-between">
      <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ mode === 'image' ? t('admin.channels.form.imageTiers') : t('admin.channels.form.requestTiers') }}
      </label>
      <button type="button" @click="addTier" class="text-xs text-primary-600 hover:text-primary-700">
        + {{ t('admin.channels.form.addTier') }}
      </button>
    </div>
    <div v-if="entry.intervals.length > 0" class="mt-2 space-y-2">
      <IntervalRow
        v-for="(iv, idx) in entry.intervals"
        :key="idx"
        :interval="iv"
        :mode="mode"
        @update="updateInterval(idx, $event)"
        @remove="removeInterval(idx)"
      />
    </div>
    <div v-else-if="mode === 'per_request'" class="mt-2 rounded border border-dashed border-gray-300 p-3 text-center text-xs text-gray-400 dark:border-dark-500">
      {{ t('admin.channels.form.noTiersYet') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { BillingMode } from '@/api/admin/channels'
import IntervalRow from './IntervalRow.vue'
import type { IntervalFormEntry, PricingFormEntry } from './types'

const props = defineProps<{
  entry: PricingFormEntry
  mode: Extract<BillingMode, 'per_request' | 'image'>
}>()

const emit = defineEmits<{
  update: [entry: PricingFormEntry]
}>()

const { t } = useI18n()

function emitField(value: string) {
  emit('update', { ...props.entry, per_request_price: value === '' ? null : value })
}

function addTier() {
  const labels = ['1K', '2K', '4K', 'HD']
  const intervals = [...(props.entry.intervals || [])]
  intervals.push({
    min_tokens: 0,
    max_tokens: null,
    tier_label: props.mode === 'image' ? labels[intervals.length] || '' : '',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    per_request_price: null,
    input_multiplier: null,
    output_multiplier: null,
    cache_write_multiplier: null,
    cache_read_multiplier: null,
    sort_order: intervals.length,
  })
  emit('update', { ...props.entry, intervals })
}

function updateInterval(idx: number, updated: IntervalFormEntry) {
  const intervals = [...props.entry.intervals]
  intervals[idx] = updated
  emit('update', { ...props.entry, intervals })
}

function removeInterval(idx: number) {
  const intervals = [...props.entry.intervals]
  intervals.splice(idx, 1)
  emit('update', { ...props.entry, intervals })
}
</script>
