<template>
  <div class="card" data-testid="prism-browser-settings">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.features.prismBrowser.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.features.prismBrowser.description') }}
      </p>
    </div>
    <div class="space-y-5 p-6">
      <div class="flex items-center justify-between gap-4">
        <div>
          <label for="prism-browser-enabled" class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.features.prismBrowser.enabled') }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.features.prismBrowser.enabledHint') }}
          </p>
        </div>
        <Toggle id="prism-browser-enabled" v-model="enabled" />
      </div>
      <div>
        <label for="prism-browser-base-url" class="input-label">
          {{ t('admin.settings.features.prismBrowser.baseUrl') }}
        </label>
        <input id="prism-browser-base-url" v-model.trim="baseUrl" type="url" class="input"
          placeholder="http://127.0.0.1:8319/v1" :required="enabled" />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.features.prismBrowser.baseUrlHint') }}
        </p>
      </div>
      <div>
        <label for="prism-browser-api-key" class="input-label">
          {{ t('admin.settings.features.prismBrowser.apiKey') }}
        </label>
        <input id="prism-browser-api-key" v-model.trim="apiKey" type="password" class="input"
          autocomplete="new-password" :required="enabled && !apiKeyConfigured"
          :placeholder="t(apiKeyConfigured
            ? 'admin.settings.features.prismBrowser.keyConfigured'
            : 'admin.settings.features.prismBrowser.keyUnconfigured')" />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.features.prismBrowser.apiKeyHint') }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'

defineProps<{ apiKeyConfigured: boolean }>()
const enabled = defineModel<boolean>('enabled', { required: true })
const baseUrl = defineModel<string>('baseUrl', { required: true })
const apiKey = defineModel<string>('apiKey', { required: true })
const { t } = useI18n()
</script>
