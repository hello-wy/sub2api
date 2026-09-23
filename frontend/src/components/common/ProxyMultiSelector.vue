<template>
  <div ref="container" class="relative">
    <button type="button" class="input flex w-full items-center justify-between text-left" :disabled="disabled" @click="open = !open">
      <span class="truncate text-sm" :class="selected.length ? 'text-gray-900 dark:text-gray-100' : 'text-gray-500'">
        {{ selectedLabel }}
      </span>
      <Icon name="chevronDown" size="sm" :class="['shrink-0 transition-transform', open && 'rotate-180']" />
    </button>
    <div v-if="open" class="absolute z-30 mt-1 w-full rounded-lg border border-gray-200 bg-white p-2 shadow-lg dark:border-dark-600 dark:bg-dark-800">
      <input v-model="query" type="search" class="input mb-2 w-full" :placeholder="t('admin.proxies.searchProxies')" @click.stop />
      <div class="max-h-56 overflow-y-auto">
        <button v-for="proxy in filteredProxies" :key="proxy.id" type="button" class="flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-700" @click="toggleProxy(proxy.id)">
          <input type="checkbox" :checked="selected.includes(proxy.id)" tabindex="-1" readonly />
          <span class="min-w-0 flex-1 truncate">{{ proxy.name }} · {{ proxy.host }}:{{ proxy.port }}</span>
        </button>
        <p v-if="!filteredProxies.length" class="px-2 py-3 text-sm text-gray-500">{{ t('common.noOptionsFound') }}</p>
      </div>
      <p v-if="max > 0" class="mt-1 px-2 text-xs text-gray-500">{{ selected.length }}/{{ max }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { Proxy } from '@/types'

const props = withDefaults(defineProps<{ modelValue: number[]; proxies: Proxy[]; max?: number; disabled?: boolean }>(), { max: 0, disabled: false })
const emit = defineEmits<{ 'update:modelValue': [value: number[]] }>()
const { t } = useI18n()
const open = ref(false)
const query = ref('')
const container = ref<HTMLElement | null>(null)
const selected = computed(() => props.modelValue || [])
const filteredProxies = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return props.proxies
  return props.proxies.filter(proxy => `${proxy.name} ${proxy.host} ${proxy.port}`.toLowerCase().includes(q))
})
const selectedLabel = computed(() => selected.value.length ? t('admin.accounts.ipChannels.addSelected', { count: selected.value.length }) : t('admin.accounts.ipChannels.add'))
function toggleProxy(id: number) {
  const next = selected.value.includes(id) ? selected.value.filter(value => value !== id) : (props.max > 0 && selected.value.length >= props.max ? selected.value : [...selected.value, id])
  emit('update:modelValue', next)
}
function onDocumentClick(event: MouseEvent) {
  if (open.value && container.value && !container.value.contains(event.target as Node)) open.value = false
}
onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>
