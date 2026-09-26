<template>
  <BaseDialog :show="true" :title="t('admin.scheduledTests.groupTitle', { name: group.name })" width="extra-wide" @close="emit('close')">
    <div class="space-y-5">
      <div class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 text-sm leading-relaxed text-gray-700 dark:border-primary-800 dark:bg-primary-900/20 dark:text-gray-300">
        <p>{{ t('admin.scheduledTests.groupDescription') }}</p>
        <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.scheduledTests.groupShowcaseHint') }}</p>
      </div>
      <ScheduledTestsPanel :show="true" :account-id="null" :group-id="group.id" :model-options="[]" :pelican-config="defaults" embedded @preview="preview = $event" />
    </div>
    <template #footer><button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button></template>
  </BaseDialog>
  <BaseDialog :show="Boolean(preview)" :title="t('admin.accounts.pelicanTest.preview')" width="extra-wide" :z-index="70" @close="preview = null">
    <div v-if="preview" class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-dark-400">{{ group.name }} · {{ preview.pelican_config?.model_id }} · {{ formatDateTime(preview.started_at) }}</p>
      <p v-if="preview.error_message" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ preview.error_message }}</p>
      <iframe v-if="previewHtml" :srcdoc="previewHtml" sandbox="allow-scripts" referrerpolicy="no-referrer" :title="group.name" class="h-[65vh] w-full rounded-xl border border-gray-200 bg-white dark:border-dark-700" />
      <pre v-else class="max-h-[65vh] overflow-auto whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-900 dark:text-gray-100">{{ preview.error_message || preview.response_text }}</pre>
    </div>
    <template #footer><button type="button" class="btn btn-secondary" @click="preview = null">{{ t('common.close') }}</button></template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ScheduledTestsPanel from '@/components/admin/account/ScheduledTestsPanel.vue'
import { PELICAN_PROMPT } from '@/utils/intelligenceTest'
import { extractPelicanHtml } from '@/utils/pelicanHtml'
import { formatDateTime } from '@/utils/format'
import type { PelicanTestConfig, ScheduledTestResult } from '@/types'

defineProps<{ group: { id: number; name: string } }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const defaults: PelicanTestConfig = { question_kind: 'pelican', prompt: PELICAN_PROMPT, reasoning_effort: 'medium', parallel_count: 1 }
const preview = ref<ScheduledTestResult | null>(null)
const previewHtml = computed(() => extractPelicanHtml(preview.value?.response_text || ''))
</script>
