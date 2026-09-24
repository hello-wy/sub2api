<template>
  <section class="card" data-testid="codex-ticket-global-settings">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.stateTicket.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.stateTicket.description') }}
      </p>
    </div>
    <div class="space-y-5 p-6">
      <div v-if="loading" class="text-sm text-gray-500" role="status">{{ t('common.loading') }}</div>
      <template v-else>
        <div class="flex items-start justify-between gap-4">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{ t('admin.settings.stateTicket.enabled') }}</label>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.stateTicket.enabledHint') }}</p>
          </div>
          <Toggle v-model="enabled" :disabled="saving" />
        </div>
        <div class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300" for="state-ticket-harvest-proxy">
              {{ t('admin.settings.stateTicket.harvestProxy') }}
            </label>
            <input
              id="state-ticket-harvest-proxy"
              v-model.trim="harvestProxyUrl"
              type="text"
              class="input w-full max-w-2xl font-mono text-sm"
              :placeholder="t('admin.settings.stateTicket.harvestProxyPlaceholder')"
              :disabled="saving"
              autocomplete="off"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.stateTicket.harvestProxyHint') }}</p>
          </div>
          <div v-if="settings?.harvest_proxy_configured" class="flex flex-wrap items-center gap-3 text-sm">
            <span class="text-gray-600 dark:text-gray-300">{{ t('admin.settings.stateTicket.currentProxy', { address: settings.harvest_proxy_display }) }}</span>
            <label class="inline-flex items-center gap-2 text-red-600 dark:text-red-300">
              <input v-model="clearProxy" type="checkbox" class="rounded border-gray-300 text-red-600 focus:ring-red-500" :disabled="saving" />
              {{ t('admin.settings.stateTicket.clearProxy') }}
            </label>
          </div>
        </div>
      </template>
      <p v-if="error" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
      <p v-if="saved" class="text-sm text-emerald-600 dark:text-emerald-400" role="status">{{ t('admin.settings.stateTicket.saved') }}</p>
      <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
        <button type="button" class="btn btn-primary btn-sm" :disabled="loading || saving" @click="save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  getCodexTicketGlobalSettings,
  saveCodexTicketGlobalSettings,
  type CodexTicketGlobalSettings,
} from '@/api/admin/codexTickets'

const { t } = useI18n()
const settings = ref<CodexTicketGlobalSettings | null>(null)
const enabled = ref(false)
const harvestProxyUrl = ref('')
const clearProxy = ref(false)
const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const value = await getCodexTicketGlobalSettings()
    settings.value = value
    enabled.value = value.enabled
    harvestProxyUrl.value = ''
    clearProxy.value = false
  } catch (cause) {
    error.value = extractApiErrorMessage(cause, t('admin.settings.stateTicket.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value) return
  saving.value = true
  saved.value = false
  error.value = ''
  try {
    const next = await saveCodexTicketGlobalSettings({
      enabled: enabled.value,
      ...(clearProxy.value
        ? { clear_proxy: true }
        : harvestProxyUrl.value
          ? { harvest_proxy_url: harvestProxyUrl.value }
          : {}),
    })
    settings.value = next
    enabled.value = next.enabled
    harvestProxyUrl.value = ''
    clearProxy.value = false
    saved.value = true
  } catch (cause) {
    error.value = extractApiErrorMessage(cause, t('admin.settings.stateTicket.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
