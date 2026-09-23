<template>
  <BaseDialog :show="accountId !== null" :title="`测试账号 #${accountId}`" width="narrow" @close="!busy && emit('close')">
    <p class="mb-4 text-sm leading-relaxed text-gray-500">测试会发送真实模型请求并产生上游用量。提交后在后台排队执行，可到智能测试页面查看进度。</p>
    <p v-if="error" class="mb-4 text-sm text-red-600" role="alert">{{ error }}</p>
    <label v-if="selectChannel" class="mb-4 block text-sm text-gray-600 dark:text-gray-300">测试 IP 通道
      <select v-model.number="selectedChannelId" class="input mt-1.5" :disabled="busy || loadingChannels" data-testid="quick-test-channel">
        <option :value="null" disabled>{{ loadingChannels ? '正在读取 IP 通道…' : '请选择可用的 IP 通道' }}</option>
        <option v-for="channel in channels" :key="channel.id" :value="channel.id" :disabled="!channel.enabled || channel.logical_enabled === false">{{ channel.proxy?.name || `IP #${channel.id}` }} · #{{ channel.id }}{{ !channel.enabled || channel.logical_enabled === false ? ' · 已暂停' : '' }}</option>
      </select>
      <span v-if="channelError" class="mt-1 block text-xs text-red-600" role="alert">{{ channelError }}</span>
    </label>
    <label class="mb-4 block text-sm text-gray-600 dark:text-gray-300">测试模型
      <select v-model="selectedModel" class="input mt-1.5" :disabled="busy || loadingModels">
        <option value="">使用测试设置默认模型</option>
        <option v-for="model in models" :key="model.id" :value="model.id">{{ model.display_name || model.id }}</option>
      </select>
      <span v-if="loadingModels" class="mt-1 block text-xs text-gray-400">正在读取账号可用模型…</span>
    </label>
    <label class="mb-4 block text-sm text-gray-600 dark:text-gray-300">本次思考强度
      <select v-model="selectedReasoning" class="input mt-1.5" :disabled="busy" aria-label="本次思考强度">
        <option value="inherit">使用测试设置</option>
        <option v-for="option in reasoningOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
      </select>
      <span class="mt-1 block text-xs text-gray-400">所选模型和接口需支持该强度；不支持时会提示原因。</span>
    </label>
    <div class="grid gap-3">
      <button v-for="setting in settings" :key="setting.test_type" class="btn btn-secondary justify-between" :disabled="!canRun || !setting.enabled" @click="run([setting.test_type])">运行{{ setting.name || testName(setting.test_type) }} <span v-if="!setting.enabled" class="text-xs">未启用</span></button>
      <button class="btn btn-primary" :disabled="!canRun || !enabledTypes.length" @click="run(enabledTypes)">{{ busy ? '正在提交…' : '运行全部测试' }}</button>
    </div>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { intelligentTestsAPI, newTestRequestKey, type TestSetting } from '@/api/intelligentTests'
import { getAvailableModels } from '@/api/admin/accounts'
import { listAccountIPChannels } from '@/api/admin/accountChannels'
import { sortAccountIPChannels } from '@/utils/accountIPChannels'
import type { AccountIPChannel } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import { testName, isTextTestModel, testSubmissionMessage, reasoningOptions } from './display'
const props = defineProps<{ accountId: number | null; selectChannel?: boolean }>()
const emit = defineEmits<{ close: []; queued: [] }>()
const settings = ref<TestSetting[]>([]), busy = ref(false), error = ref('')
const models = ref<{ id: string; display_name?: string }[]>([]), selectedModel = ref(''), loadingModels = ref(false)
const selectedReasoning = ref('inherit')
const channels = ref<AccountIPChannel[]>([]), selectedChannelId = ref<number | null>(null), loadingChannels = ref(false), channelError = ref('')
const selectedChannel = computed(() => channels.value.find(channel => channel.id === selectedChannelId.value))
const targetAccountId = computed(() => props.selectChannel ? selectedChannelId.value : props.accountId)
const canRun = computed(() => !busy.value && !!targetAccountId.value && !loadingChannels.value && !channelError.value && (!props.selectChannel || (!!selectedChannel.value?.enabled && selectedChannel.value.logical_enabled !== false)))
const app = useAppStore()
const enabledTypes = computed(() => settings.value.filter(item => item.enabled).map(item => item.test_type))
let key = '', signature = '', generation = 0, modelGeneration = 0
let channelController: AbortController | undefined
async function loadModels(id: number | null, current: number) {
  const revision = ++modelGeneration
  models.value = []; selectedModel.value = ''
  if (!id) { loadingModels.value = false; return }
  loadingModels.value = true
  try {
    const result = await getAvailableModels(id)
    if (current === generation && revision === modelGeneration) models.value = result.filter(model => isTextTestModel(model.id))
  } catch { /* Model discovery is optional; saved test models remain usable. */ }
  finally { if (current === generation && revision === modelGeneration) loadingModels.value = false }
}
watch(selectedChannelId, id => { if (props.selectChannel) void loadModels(id, generation) })
watch(() => [props.accountId, props.selectChannel] as const, async ([id]) => {
  const current = ++generation
  channelController?.abort(); modelGeneration++
  channels.value = []; selectedChannelId.value = null; channelError.value = ''; loadingChannels.value = false; loadingModels.value = false
  key = ''; signature = ''; error.value = ''; settings.value = []; models.value = []; selectedModel.value = ''; selectedReasoning.value = 'inherit'
  if (id === null) return
  try {
    // Settings are required to render the test actions; model discovery is
    // only a convenience and must never block the dialog when an upstream
    // catalog is slow or unavailable.
    const settingsPromise = intelligentTestsAPI.settings().then(result => {
      if (current === generation) settings.value = result
    })
    let routePromise: Promise<void>
    if (props.selectChannel) {
      const controller = new AbortController(); channelController = controller; loadingChannels.value = true
      routePromise = listAccountIPChannels(id, controller.signal).then(result => {
        if (current !== generation || controller.signal.aborted) return
        channels.value = sortAccountIPChannels(result)
        selectedChannelId.value = channels.value.find(channel => channel.enabled && channel.logical_enabled !== false)?.id ?? null
        if (!selectedChannelId.value) channelError.value = '没有已启用的 IP 通道，请先检查通道状态。'
      }).catch(() => { if (current === generation && !controller.signal.aborted) channelError.value = '无法读取 IP 通道，请关闭后重试。' })
        .finally(() => { if (current === generation) loadingChannels.value = false })
    } else routePromise = loadModels(id, current)
    await Promise.all([settingsPromise, routePromise])
  }
  catch (err) { if (current === generation) error.value = extractApiErrorMessage(err, '无法读取测试设置') }
}, { immediate: true })
onUnmounted(() => { generation++; modelGeneration++; channelController?.abort() })
async function run(types: string[]) {
  if (!canRun.value || !targetAccountId.value) return
  const accountId = targetAccountId.value
  const current = generation
  types = [...types].sort()
  const nextSignature = JSON.stringify([accountId, types, selectedModel.value, selectedReasoning.value])
  if (signature !== nextSignature) { key = newTestRequestKey(); signature = nextSignature }
  busy.value = true; error.value = ''
  try {
    const overrides = selectedModel.value ? Object.fromEntries(types.map(type => [type, selectedModel.value])) : undefined
    const reasoning = selectedReasoning.value === 'inherit' ? undefined : Object.fromEntries(types.map(type => [type, selectedReasoning.value]))
    const data = reasoning ? await intelligentTestsAPI.run([accountId], types, key, overrides, reasoning) : overrides ? await intelligentTestsAPI.run([accountId], types, key, overrides) : await intelligentTestsAPI.run([accountId], types, key)
    app.showSuccess(testSubmissionMessage(data)); emit('queued')
    if (current === generation) { signature = ''; emit('close') }
  }
  catch (err) { if (current === generation) error.value = extractApiErrorMessage(err, '任务提交失败，可重试') }
  finally { busy.value = false }
}
</script>
