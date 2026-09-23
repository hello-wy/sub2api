<template>
  <BaseDialog :show="show" :title="t('accountTicketDefaults.title')" :show-close-button="!saving" :close-on-escape="!saving" @close="close">
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('accountTicketDefaults.scope') }}</p>
      <p class="text-xs text-gray-500">{{ t('accountTicketDefaults.futureOnly') }}</p>
      <p v-if="loading" role="status" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
      <label class="block">
        <span class="input-label">{{ t('accountTicketDefaults.plan') }}</span>
        <select v-model="choice" class="input" :disabled="!loaded || loading || saving" data-testid="ticket-defaults-plan">
          <option value="off">{{ t('accountTicketDefaults.off') }}</option>
          <option value="pro">{{ t('accountTicketDefaults.pro') }}</option>
          <option value="team">{{ t('accountTicketDefaults.team') }}</option>
        </select>
      </label>
      <div v-if="loaded && draft.enabled && !prerequisitesReady" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" data-testid="ticket-defaults-prerequisites">
        <p>{{ t(prerequisitesError ? 'accountTicketDefaults.prerequisitesUnknown' : 'accountTicketDefaults.prerequisites') }}</p>
        <a href="/admin/settings?tab=gateway" target="_blank" rel="noopener noreferrer" class="mt-2 inline-block underline">{{ t('accountTicketDefaults.settings') }}</a>
        <button type="button" class="ml-3 underline" :disabled="loadingPrerequisites || saving" @click="loadPrerequisites">{{ t('accountTicketDefaults.retryPrerequisites') }}</button>
      </div>
      <p v-if="error" role="alert" class="text-sm text-red-600" data-testid="ticket-defaults-error">{{ error }}</p>
      <button v-if="!loaded && !loading" type="button" class="btn btn-secondary" data-testid="ticket-defaults-reload" @click="load">{{ t('accountTicketDefaults.reload') }}</button>
      <p v-if="saved" role="status" class="text-sm text-emerald-700 dark:text-emerald-300" data-testid="ticket-defaults-saved">{{ t('accountTicketDefaults.saved') }}</p>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.close') }}</button>
        <button type="button" class="btn btn-primary" :disabled="!canSave" data-testid="ticket-defaults-save" @click="save">{{ saving ? t('common.loading') : t('accountTicketDefaults.save') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getAccountTicketDefaults, saveAccountTicketDefaults, type AccountTicketDefaults } from '@/api/admin/accountTicketDefaults'
import { getCodexTicketGlobalSettings, type CodexTicketGlobalSettings } from '@/api/admin/codexTickets'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: []; saved: [value: AccountTicketDefaults] }>()
const { t } = useI18n()
const draft = ref<AccountTicketDefaults>({ enabled: false, ticket_plan: 'pro' })
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const saved = ref(false)
const choice = computed<'off' | 'pro' | 'team'>({
  get: () => draft.value.enabled ? draft.value.ticket_plan : 'off',
  set: value => {
    draft.value = { enabled: value !== 'off', ticket_plan: value === 'off' ? draft.value.ticket_plan : value }
    saved.value = false
  }
})
const error = ref('')
const prerequisites = ref<CodexTicketGlobalSettings | null>(null)
const prerequisitesError = ref(false)
const loadingPrerequisites = ref(false)
const prerequisitesReady = computed(() => !!prerequisites.value?.enabled && !!prerequisites.value?.harvest_proxy_configured)
const canSave = computed(() => props.show && loaded.value && !loading.value && !saving.value)
let generation = 0
let readController: AbortController | undefined
let prerequisitesController: AbortController | undefined

async function loadPrerequisites() {
  if (!props.show || saving.value) return
  prerequisitesController?.abort()
  const current = new AbortController()
  prerequisitesController = current
  const token = generation
  loadingPrerequisites.value = true
  prerequisitesError.value = false
  prerequisites.value = null
  try {
    const value = await getCodexTicketGlobalSettings(current.signal)
    if (!current.signal.aborted && generation === token) prerequisites.value = value
  } catch {
    if (!current.signal.aborted && generation === token) prerequisitesError.value = true
  } finally {
    if (generation === token && prerequisitesController === current) loadingPrerequisites.value = false
  }
}

async function load() {
  if (!props.show || saving.value) return
  readController?.abort()
  const current = new AbortController()
  readController = current
  const token = generation
  loading.value = true
  loaded.value = false
  error.value = ''
  try {
    const value = await getAccountTicketDefaults(current.signal)
    if (current.signal.aborted || generation !== token) return
    draft.value = { enabled: value.enabled, ticket_plan: value.ticket_plan }
    loaded.value = true
  } catch (cause) {
    if (!current.signal.aborted && generation === token) error.value = extractApiErrorMessage(cause instanceof Error ? null : cause, t('accountTicketDefaults.loadFailed'))
  } finally {
    if (generation === token && readController === current) loading.value = false
  }
}

async function save() {
  if (!canSave.value) return
  const token = generation
  saving.value = true
  saved.value = false
  error.value = ''
  try {
    const value = await saveAccountTicketDefaults({ ...draft.value })
    if (generation !== token || !props.show) return
    draft.value = { enabled: value.enabled, ticket_plan: value.ticket_plan }
    saved.value = true
    emit('saved', value)
  } catch (cause) {
    if (generation === token && props.show) error.value = extractApiErrorMessage(cause instanceof Error ? null : cause, t('accountTicketDefaults.saveFailed'))
  } finally {
    if (generation === token) saving.value = false
  }
}

function close() {
  if (!saving.value) emit('close')
}

function invalidate() {
  generation++
  readController?.abort()
  prerequisitesController?.abort()
}

watch(() => props.show, show => {
  invalidate()
  saving.value = false
  loading.value = false
  loaded.value = false
  saved.value = false
  error.value = ''
  draft.value = { enabled: false, ticket_plan: 'pro' }
  if (show) {
    void load()
    void loadPrerequisites()
  }
}, { immediate: true })
onBeforeUnmount(invalidate)
</script>
