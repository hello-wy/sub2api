<template>
  <BaseDialog
    :show="show"
    :title="t('admin.channelMonitor.sortOrder.title')"
    :show-close-button="!saving"
    :close-on-escape="!saving"
    width="normal"
    @close="close"
  >
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.channelMonitor.sortOrder.hint') }}</p>
      <p v-if="loading" role="status" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
      <div v-else-if="loadFailed" role="alert" class="space-y-3">
        <p class="text-sm text-red-500">{{ t('admin.channelMonitor.loadError') }}</p>
        <button type="button" class="btn btn-secondary" @click="loadMonitors">{{ t('common.refresh') }}</button>
      </div>
      <p v-else-if="items.length === 0" class="text-sm text-gray-500">{{ t('admin.channelMonitor.noMonitorsYet') }}</p>
      <VueDraggable v-else v-model="items" :animation="200" handle=".monitor-drag-handle" :disabled="saving" class="space-y-2">
        <div
          v-for="(item, index) in items"
          :key="item.id"
          :data-monitor-id="item.id"
          class="flex items-center gap-3 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-700"
        >
          <button type="button" class="monitor-drag-handle cursor-grab touch-none text-gray-400 active:cursor-grabbing" :aria-label="t('admin.channelMonitor.sortOrder.drag')" :disabled="saving">
            <Icon name="menu" size="md" />
          </button>
          <div class="min-w-0 flex-1">
            <div class="break-words font-medium text-gray-900 dark:text-white">{{ item.name }}</div>
            <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span :class="providerBadgeClass(item.provider)" class="rounded-md px-2 py-0.5">{{ providerLabel(item.provider) }}</span>
              <span v-if="item.group_name">{{ item.group_name }}</span>
              <span v-if="!item.enabled">{{ t('common.disabled') }}</span>
            </div>
          </div>
          <div class="flex gap-1">
            <button type="button" class="btn btn-ghost p-1.5" :disabled="saving || index === 0" :aria-label="t('admin.channelMonitor.sortOrder.moveUp', { name: item.name })" @click="move(index, -1)">
              <Icon name="chevronUp" size="sm" />
            </button>
            <button type="button" class="btn btn-ghost p-1.5" :disabled="saving || index === items.length - 1" :aria-label="t('admin.channelMonitor.sortOrder.moveDown', { name: item.name })" @click="move(index, 1)">
              <Icon name="chevronDown" size="sm" />
            </button>
          </div>
        </div>
      </VueDraggable>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" data-testid="save-monitor-order" :disabled="saving || loading || loadFailed || items.length === 0" @click="save">
          {{ saving ? t('common.submitting') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const { providerLabel, providerBadgeClass } = useChannelMonitorFormat()
const items = ref<ChannelMonitor[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const saving = ref(false)
let controller: AbortController | null = null

async function loadMonitors() {
  controller?.abort()
  const ctrl = new AbortController()
  controller = ctrl
  loading.value = true
  loadFailed.value = false
  items.value = []
  try {
    // Load every page without the main table's filters, including disabled monitors.
    const all: ChannelMonitor[] = []
    for (let page = 1; ; page++) {
      const result = await adminAPI.channelMonitor.list({ page, page_size: 100 }, { signal: ctrl.signal })
      if (ctrl.signal.aborted) return
      all.push(...result.items)
      if (all.length >= result.total) break
      if (result.items.length === 0 || result.total > 10000) throw new Error('Incomplete monitor list')
    }
    items.value = all.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
  } catch {
    if (!ctrl.signal.aborted) loadFailed.value = true
  } finally {
    if (controller === ctrl) loading.value = false
  }
}

function move(index: number, direction: number) {
  const next = index + direction
  if (saving.value || next < 0 || next >= items.value.length) return
  const [item] = items.value.splice(index, 1)
  items.value.splice(next, 0, item)
}

function close() {
  if (!saving.value) emit('close')
}

async function save() {
  if (saving.value || loading.value || loadFailed.value || items.value.length === 0) return
  saving.value = true
  try {
    await adminAPI.channelMonitor.updateSortOrder(items.value.map(item => item.id))
    appStore.showSuccess(t('admin.channelMonitor.sortOrder.saved'))
    emit('close')
    emit('saved')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.sortOrder.saveError')))
  } finally {
    saving.value = false
  }
}

watch(() => props.show, show => {
  if (show) void loadMonitors()
  else {
    controller?.abort()
    items.value = []
  }
}, { immediate: true })
onUnmounted(() => controller?.abort())
</script>
