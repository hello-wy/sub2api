<template>
  <article class="min-w-0 rounded-3xl border border-gray-200 bg-white p-5 dark:border-dark-600 dark:bg-dark-800 sm:p-6" :aria-label="group.group_name">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h2 class="break-words text-lg font-semibold text-gray-950 dark:text-white">{{ group.group_name }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ group.platform }}</p>
      </div>
      <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ring-1 ring-inset" :class="stateBadge(group.status)">
        {{ t(`groupStatus.status.${serviceState(group.status)}`) }}
      </span>
    </div>

    <div class="mt-6" :aria-label="t('groupStatus.realTraffic')">
      <dl class="grid grid-cols-3 gap-2">
        <div>
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupStatus.availability') }}</dt>
          <dd class="mt-1.5 whitespace-nowrap text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-testid="availability">{{ formatAvailability(group.metrics.success_rate) }}</dd>
        </div>
        <div :title="t('groupStatus.speedHint')">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupStatus.generationSpeed') }}</dt>
          <dd class="mt-1.5 whitespace-nowrap text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-testid="generation-speed">{{ formatGenerationSpeed(group.metrics.output_tokens_per_second) }}</dd>
        </div>
        <div :title="t('groupStatus.latencyHint')">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('groupStatus.firstToken') }}</dt>
          <dd class="mt-1.5 whitespace-nowrap text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100" data-testid="first-token">{{ formatGroupLatency(group.metrics.ttft_ms) }}</dd>
        </div>
      </dl>
    </div>

    <div class="mt-6">
      <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">{{ t('groupStatus.history') }}</p>
      <div v-if="group.buckets.length" class="flex h-8 gap-0.5" :aria-label="t('groupStatus.history')">
        <span
          v-for="bucket in group.buckets"
          :key="bucket.start"
          class="min-w-0 flex-1 rounded-full transition-opacity hover:opacity-70 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
          :class="stateColor(bucket.status)"
          :title="bucketLabel(bucket)"
          :aria-label="bucketLabel(bucket)"
          role="img"
          tabindex="0"
        />
      </div>
      <div v-else class="flex h-8 items-center rounded-lg bg-gray-100 px-3 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">{{ t('groupStatus.historyEmpty') }}</div>
    </div>

    <div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
      <div class="min-w-0 text-xs text-gray-500 dark:text-gray-400" data-testid="probe-summary">
        <div class="flex flex-wrap items-center gap-2">
          <span>{{ t('groupStatus.probe.recent') }}</span>
          <span v-if="group.last_probe" class="font-medium" :class="probeColor">{{ t(`groupStatus.probe.status.${group.last_probe.status}`) }}</span>
          <span v-else>{{ t('groupStatus.probe.notRun') }}</span>
        </div>
        <p v-if="group.last_probe" class="mt-1 break-words">
          {{ formatGroupTime(group.last_probe.checked_at, locale) }} · {{ group.last_probe.model }}
        </p>
      </div>
      <button v-if="admin" type="button" class="btn btn-secondary btn-sm shrink-0 text-xs" data-testid="manage-probe" @click="emit('manage', group)">
        {{ t('groupStatus.probe.manage') }}
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupServiceStatus, GroupStatusBucket } from '@/api/groupStatus'
import { formatAvailability, formatGenerationSpeed, formatGroupLatency, formatGroupTime, serviceState, stateBadge, stateColor } from './format'

const props = defineProps<{ group: GroupServiceStatus; admin: boolean }>()
const emit = defineEmits<{ manage: [group: GroupServiceStatus] }>()
const { t, locale } = useI18n()
const probeColor = computed(() => {
  if (props.group.last_probe?.status === 'success') return 'text-emerald-700 dark:text-emerald-400'
  if (props.group.last_probe?.status === 'failed') return 'text-rose-700 dark:text-rose-400'
  return 'text-gray-600 dark:text-gray-300'
})
function bucketLabel(bucket: GroupStatusBucket) {
  return t('groupStatus.bucket', {
    start: formatGroupTime(bucket.start, locale.value),
    end: formatGroupTime(bucket.end, locale.value),
    status: t(`groupStatus.status.${serviceState(bucket.status)}`),
    rate: formatAvailability(bucket.success_rate),
  })
}
</script>
