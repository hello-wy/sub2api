<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import { listCodexGateways } from '@/api/admin/accounts'

const props = defineProps<{ active: boolean; inherited?: boolean }>()
const model = defineModel<string>({ default: '' })
const { t } = useI18n()
const gateways = ref<string[]>([])
const loading = ref(false)
const failed = ref(false)
const options = computed(() => [
  { value: '', label: t('admin.accounts.codexGateway.official') },
  ...Array.from(new Set([...gateways.value, model.value].filter(Boolean)))
    .map(value => ({ value, label: value }))
])

async function load() {
  if (props.inherited || loading.value) return
  loading.value = true
  failed.value = false
  try {
    gateways.value = await listCodexGateways()
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

watch(() => [props.active, props.inherited], () => {
  if (props.active) void load()
}, { immediate: true })
</script>

<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.codexGateway.label') }}</label>
    <p v-if="inherited" class="text-sm text-gray-500 dark:text-dark-400">
      {{ t('admin.accounts.codexGateway.inherited') }}
    </p>
    <template v-else>
      <Select
        :model-value="model"
        :options="options"
        :aria-label="t('admin.accounts.codexGateway.label')"
        :searchable="true"
        :creatable="true"
        :clearable="true"
        :loading="loading"
        :creatable-prefix="t('admin.accounts.codexGateway.useNew')"
        :search-placeholder="t('admin.accounts.codexGateway.searchPlaceholder')"
        @update:model-value="model = typeof $event === 'string' ? $event : ''"
      />
      <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
        {{ t('admin.accounts.codexGateway.hint') }}
      </p>
      <p v-if="model" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
        {{ t('admin.accounts.codexGateway.credentialsHint') }}
      </p>
      <button v-if="failed" type="button" class="mt-1 text-xs text-primary-600 hover:underline" @click="load">
        {{ t('admin.accounts.codexGateway.retry') }}
      </button>
    </template>
  </div>
</template>
