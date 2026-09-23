<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-3 text-sm">
      <span class="text-gray-500">已选 {{ selected.length }} 条 · 排队或运行中的任务暂不可删除</span>
      <button class="btn btn-secondary text-red-600" :disabled="busy || !selected.length" @click="askDelete(selected)">删除所选记录</button>
    </div>
    <div class="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
    <table class="w-full whitespace-nowrap text-left text-sm">
      <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900"><tr><th class="p-4"><input type="checkbox" class="rounded" aria-label="选择本页可删除记录" :checked="allSelected" :disabled="busy || !removable.length" @change="selectPage" /></th><th class="p-4">测试时间</th><th class="p-4">账号</th><th class="p-4">测试类型</th><th class="p-4">结果</th><th class="p-4">耗时</th><th class="p-4">模型</th><th class="p-4">操作</th></tr></thead>
      <tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="record in records" :key="record.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/40">
        <td class="p-4"><input v-model="selected" type="checkbox" class="rounded" :value="record.id" :aria-label="`选择记录 ${record.id}`" :disabled="busy || isPending(record.status)" /></td>
        <td class="p-4 text-gray-500">{{ testTime(record.created_at) }}</td><td class="p-4 font-medium">#{{ record.account_id }}</td><td class="p-4">{{ testName(record.test_type) }}</td>
        <td class="space-y-2 p-4"><button v-if="record.test_type === 'pelican'" class="block w-48 overflow-hidden rounded-lg" aria-label="查看生成的鹈鹕图像" @click="emit('detail', record.id)"><TestGeneratedImage v-if="!isPending(record.status)" :source="record.result_image" :record-id="record.id" /><span v-else class="block p-4 text-xs text-gray-500">正在生成图像…</span></button><template v-else><TestStatusBadge :status="record.status" /><TestAssessment :assessment="record.evaluation" :status="record.status" compact /></template></td><td class="p-4 tabular-nums">{{ isPending(record.status) ? '尚未完成' : `${(record.duration_ms / 1000).toFixed(1)} 秒` }}</td><td class="max-w-48 truncate p-4">{{ record.model || '默认' }}</td>
        <td class="space-x-3 p-4"><button class="text-primary-600 hover:underline" @click="emit('detail', record.id)">查看详情</button><button class="text-red-600 hover:underline disabled:opacity-40" :aria-label="`删除记录 ${record.id}`" :disabled="busy || isPending(record.status)" @click="askDelete([record.id])">删除</button></td>
      </tr></tbody>
    </table>
    <p v-if="!records.length" class="py-16 text-center text-sm text-gray-500">没有符合条件的测试记录</p>
    </div>
    <BaseDialog :show="pending.length > 0" title="删除测试记录" width="narrow" :close-on-click-outside="!busy" :close-on-escape="!busy" @close="close">
      <p class="text-sm leading-relaxed text-gray-600 dark:text-gray-300">确定删除这 {{ pending.length }} 条记录？对应的答复、图像和公开展示会一并移除，删除后无法恢复。账号和定时测试设置会保留。</p>
      <p v-if="error" role="alert" class="mt-3 text-sm text-red-600">{{ error }}</p>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" :disabled="busy" @click="close">取消</button><button class="btn bg-red-600 text-white hover:bg-red-700" :disabled="busy" data-testid="confirm-delete-records" @click="confirmDelete">{{ busy ? '删除中…' : '确认删除' }}</button></div></template>
    </BaseDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { intelligentTestsAPI, type TestRecord } from '@/api/intelligentTests'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TestStatusBadge from './TestStatusBadge.vue'
import TestAssessment from './TestAssessment.vue'
import TestGeneratedImage from './TestGeneratedImage.vue'
import { testName, testTime, isPending } from './display'
const props = defineProps<{ records: TestRecord[] }>()
const emit = defineEmits<{ detail: [id: number]; deleted: [ids: number[]]; busy: [value: boolean] }>()
const app = useAppStore(), selected = ref<number[]>([]), pending = ref<number[]>([]), busy = ref(false), error = ref('')
onBeforeUnmount(() => { if (busy.value) emit('busy', false) })
const removable = computed(() => props.records.filter(record => !isPending(record.status)).map(record => record.id))
const allSelected = computed(() => removable.value.length > 0 && removable.value.every(id => selected.value.includes(id)))
watch(removable, ids => { selected.value = selected.value.filter(id => ids.includes(id)) })
function selectPage() { selected.value = allSelected.value ? [] : [...removable.value] }
function askDelete(ids: number[]) { if (!busy.value) { pending.value = ids.filter(id => removable.value.includes(id)); error.value = '' } }
function close() { if (!busy.value) pending.value = [] }
async function confirmDelete() {
  if (busy.value || !pending.value.length) return
  const ids = [...pending.value]
  busy.value = true; error.value = ''; emit('busy', true)
  try {
    const result = ids.length === 1 ? await intelligentTestsAPI.deleteRecord(ids[0]!) : await intelligentTestsAPI.deleteRecords(ids)
    selected.value = selected.value.filter(id => !ids.includes(id)); pending.value = []
    app.showSuccess(`已删除 ${result.deleted} 条测试记录`)
    emit('deleted', ids)
  } catch (failure) { error.value = extractApiErrorMessage(failure, '删除失败，请重试') }
  finally { busy.value = false; emit('busy', false) }
}
</script>
