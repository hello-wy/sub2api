<template>
  <AppLayout>
    <section class="flex h-[calc(100dvh-8rem)] min-h-0 flex-col gap-3">
      <p v-if="error" role="alert" class="text-sm text-red-500">{{ error }}</p>
      <button v-if="!loading && canvas.settings.enabled && (error || !selectedKeyId)" type="button" class="btn btn-secondary btn-sm self-start" @click="() => openKeyPicker()">重新选择密钥</button>
      <p v-if="!loading && !canvas.settings.enabled" class="text-sm">管理员尚未开启无限画布。</p>
      <p v-else-if="!loading && !canvas.settings.url" class="text-sm">尚未配置画布地址，请在 Sub2API 管理设置中填写画布地址。</p>
      <p v-else-if="!loading && !frameUrl" role="alert" class="text-sm text-red-500">画布地址无效，请在功能开关中检查画布地址。</p>
      <p v-else-if="!loading && !keys.length" class="text-sm">没有可用于画布的 API Key，请在密钥管理中配置有效密钥及分组。</p>
      <iframe v-if="canvas.settings.enabled && frameUrl && selectedKeyId && !loading" :key="frameVersion" ref="frame" :src="frameUrl"
        title="无限画布" class="min-h-0 w-full flex-1 border-0" allowfullscreen referrerpolicy="origin" />
      <BaseDialog :show="showKeyPicker" :title="keyPickerTitle" width="normal" @close="closeKeyPicker">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ selectedKeyId ? '确认后将重新连接画布，请先结束正在进行的生成或 Agent 对话。已提交的生成任务不保证取消。' : '请选择你自己的有效 API Key，之后会记住本次选择。' }}</p>
        <p class="mt-3 break-all text-xs text-gray-500 dark:text-gray-400">API 站点：{{ configuredBaseUrl }}（管理员统一设置）</p>
        <p v-if="keysLoading" role="status" class="mt-4 text-sm">正在读取可用密钥…</p>
        <p class="mt-3 text-sm font-medium">我的密钥 · 仅列出当前账号的可用密钥</p>
        <div v-if="!keysLoading" class="mt-4 max-h-72 space-y-2 overflow-y-auto">
          <label v-for="key in keys" :key="key.id" class="flex cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-3 hover:border-primary-500 dark:border-dark-600">
            <input v-model="pendingKeyId" type="radio" name="canvas-key" :value="key.id" :disabled="keySwitching" />
            <span class="min-w-0 flex-1"><span class="block truncate font-medium text-gray-900 dark:text-white">{{ key.name }} <span class="text-xs font-normal text-gray-500">#{{ key.id }}{{ key.id === currentKeyId ? ' · 当前使用' : '' }}</span></span><span v-if="key.group?.name" class="block text-xs text-gray-500">{{ key.group.name }}</span></span>
          </label>
          <p v-if="!keys.length && !pickerError" class="text-sm text-gray-500">暂无已分组的有效 API Key，请先在密钥管理中配置。</p>
        </div>
        <p v-if="pickerError" role="alert" class="mt-4 text-sm text-red-500">{{ pickerError }}</p>
        <template #footer>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <RouterLink to="/keys" class="text-sm text-primary-600">管理密钥</RouterLink>
            <div class="flex gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="closeKeyPicker">取消</button>
              <button type="button" class="btn btn-primary btn-sm" :disabled="!pendingKeyId || keysLoading || keySwitching" @click="confirmKey">{{ keySwitching ? '正在验证…' : selectedKeyId ? '确认切换' : '进入画布' }}</button>
            </div>
          </div>
        </template>
      </BaseDialog>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { getCurrentUser } from '@/api/auth'
import { keysAPI } from '@/api/keys'
import { apiClient, buildGatewayUrl } from '@/api/client'
import { useCanvasStore } from '@/stores/canvas'
import type { ApiKey } from '@/types'
import { CANVAS_PROTOCOL, buildCanvasUrl, trustedCanvasMessage } from '@/utils/canvas-bridge'
import { detectTheme } from '@/utils/embedded-url'

type CanvasSession = {
  user: { id: number; role: 'admin' | 'user' }; apiKey: string; keyId: number; keyName: string; canvasUrl: string; baseUrl: string; upstreamRetryCount?: number
}
const auth = useAuthStore()
const canvas = useCanvasStore()
const router = useRouter()
const { locale } = useI18n()
const frame = ref<HTMLIFrameElement | null>(null)
const keys = ref<ApiKey[]>([])
const selectedKeyId = ref(0)
const pendingKeyId = ref(0)
const keyPickerPurpose = ref<'image' | 'text' | null>(null)
const currentKeyId = ref(0)
const keyPickerTitle = computed(() => {
  if (keyPickerPurpose.value === 'image') return selectedKeyId.value ? '切换图片生成密钥' : '选择图片生成密钥'
  if (keyPickerPurpose.value === 'text') return selectedKeyId.value ? '切换语言模型密钥' : '选择语言模型密钥'
  return selectedKeyId.value ? '切换画布密钥' : '选择画布 API Key'
})
const showKeyPicker = ref(false)
const loading = ref(false)
const keysLoading = ref(false)
const keySwitching = ref(false)
const imageKeyId = ref(0)
const textKeyId = ref(0)
const error = ref('')
const pickerError = ref('')
const frameVersion = ref(0)
let version = 0
let pickerVersion = 0
const initialTheme = detectTheme()
const initialLocale = locale.value
const configuredBaseUrl = computed(() => (canvas.settings.effective_api_base_url || buildGatewayUrl('/')).replace(/\/+$/, ''))
const frameUrl = computed(() => {
  try { const url = canvas.settings.url; return url ? buildCanvasUrl(url, initialTheme, initialLocale) : '' }
  catch { return '' }
})

async function fetchAvailableKeys(): Promise<ApiKey[]> {
  const all: ApiKey[] = []
  let page = 1
  let pages = 1
  do {
    const result = await keysAPI.list(page, 10, { status: 'active' })
    all.push(...result.items)
    pages = Math.ceil(result.total / result.page_size)
    page++
  } while (page <= pages)
  return all.filter(key => key.user_id === auth.user?.id && key.status === 'active' && key.group_id != null && (!key.quota || key.quota_used < key.quota) && (!key.expires_at || Date.parse(key.expires_at) > Date.now()))
}

async function openKeyPicker(purpose?: 'image' | 'text') {
  const requestVersion = ++pickerVersion
  const userId = auth.user?.id
  keyPickerPurpose.value = purpose || null
  currentKeyId.value = (purpose === 'image' ? imageKeyId.value : purpose === 'text' ? textKeyId.value : 0) || selectedKeyId.value
  showKeyPicker.value = true
  keysLoading.value = true
  pickerError.value = ''
  pendingKeyId.value = 0
  try {
    const available = await fetchAvailableKeys()
    if (requestVersion !== pickerVersion || userId !== auth.user?.id) return
    keys.value = available
    pendingKeyId.value = available.some(key => key.id === currentKeyId.value) ? currentKeyId.value : available[0]?.id || 0
  } catch {
    if (requestVersion === pickerVersion) pickerError.value = '无法读取可用密钥，请关闭弹窗后重试。'
  } finally { if (requestVersion === pickerVersion) keysLoading.value = false }
}
function closeKeyPicker() {
  pickerVersion++
  keyPickerPurpose.value = null
  showKeyPicker.value = false
  keysLoading.value = false
  keySwitching.value = false
}
function validateSession(session: CanvasSession, keyId: number): string {
  if (session.user.id !== auth.user?.id || session.keyId !== keyId || session.canvasUrl !== canvas.settings.url || !session.apiKey) throw new Error('画布身份或配置已变化')
  const baseUrl = (session.baseUrl || buildGatewayUrl('/')).replace(/\/+$/, '')
  const url = new URL(baseUrl)
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error('画布 API 站点无效，请联系管理员')
  return baseUrl
}
async function confirmKey() {
  const key = keys.value.find(item => item.id === pendingKeyId.value)
  const userId = auth.user?.id
  const purpose = keyPickerPurpose.value
  if (!key || !userId || keysLoading.value || keySwitching.value) return
  const requestVersion = pickerVersion
  const sessionVersion = version
  keySwitching.value = true
  pickerError.value = ''
  try {
    const { data: session } = await apiClient.get<CanvasSession>('/canvas/session/' + key.id)
    if (requestVersion !== pickerVersion || sessionVersion !== version || userId !== auth.user?.id) return
    validateSession(session, key.id)
    if (purpose === 'image') {
      localStorage.setItem(`sub2api:canvas-image-key:${userId}`, String(key.id))
      imageKeyId.value = key.id
    } else if (purpose === 'text') {
      localStorage.setItem(`sub2api:canvas-text-key:${userId}`, String(key.id))
      textKeyId.value = key.id
    } else {
      localStorage.setItem(`sub2api:canvas-key:${userId}`, String(key.id))
      selectedKeyId.value = key.id
    }
    invalidateFrame()
    error.value = ''
    closeKeyPicker()
    frameVersion.value++
  } catch {
    if (requestVersion === pickerVersion && sessionVersion === version) pickerError.value = '密钥验证失败，请检查分组、状态、有效期和配额后重试。当前选择未改变。'
  } finally { if (requestVersion === pickerVersion) keySwitching.value = false }
}

async function onMessage(event: MessageEvent) {
  const target = frame.value?.contentWindow || null
  if (!trustedCanvasMessage(event, target, frameUrl.value)) return
  const request = event.data
  if (request.type === 'select-key') {
    if (canvas.settings.enabled && !showKeyPicker.value) void openKeyPicker(request.purpose)
    return
  }
  if (request.type === 'reconnect') {
    invalidateFrame()
    frameVersion.value++
    return
  }
  if (request.type !== 'session' && request.type !== 'navigate') return
  const requestVersion = version
  const selectedId = selectedKeyId.value
  try {
    if (request.type === 'navigate') {
      const { data: user } = await getCurrentUser()
      if (requestVersion !== version || user.id !== auth.user?.id) return
      const paths = ['/keys', '/usage', ...(user.role === 'admin' ? ['/admin/accounts', '/admin/groups', '/admin/settings', '/admin/settings?tab=features', '/admin/users'] : [])]
      if (paths.includes(request.path)) void router.push(request.path)
      return
    }
    const imageId = imageKeyId.value || selectedId
    const textId = textKeyId.value || selectedId
    const sessions = new Map(await Promise.all([...new Set([selectedId, imageId, textId])].map(async id => {
      const { data: session } = await apiClient.get<CanvasSession>('/canvas/session/' + id)
      validateSession(session, id)
      return [id, session] as const
    })))
    if (requestVersion !== version || selectedId !== selectedKeyId.value || target !== frame.value?.contentWindow) return
    const session = sessions.get(selectedId)!
    const baseUrl = validateSession(session, selectedId)
    const sessionKey = (id: number) => {
      const key = sessions.get(id)!
      if (key.user.role !== session.user.role || validateSession(key, id) !== baseUrl) throw new Error('画布身份或配置已变化')
      return { apiKey: key.apiKey, keyId: key.keyId, keyName: key.keyName }
    }
    error.value = ''
    target?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'session', requestId: request.requestId, session: { ...session, baseUrl, imageKey: sessionKey(imageId), textKey: sessionKey(textId) } }, event.origin)
  } catch {
    if (requestVersion !== version || selectedId !== selectedKeyId.value || target !== frame.value?.contentWindow) return
    error.value = '画布身份验证失败，请检查画布开关、登录状态和所选密钥，或重新选择密钥。'
    target?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'session', requestId: request.requestId, error: error.value }, event.origin)
  }
}

function invalidateFrame() {
  version++
  loading.value = false
  if (frameUrl.value) frame.value?.contentWindow?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'invalidate' }, new URL(frameUrl.value).origin)
}
async function initialize() {
  const requestVersion = ++version
  loading.value = true
  error.value = ''
  try {
    await canvas.fetchSettings()
    if (requestVersion !== version || !auth.user?.id || !canvas.settings.enabled) return
    const available = await fetchAvailableKeys()
    if (requestVersion !== version) return
    keys.value = available
    const saved = Number(localStorage.getItem(`sub2api:canvas-key:${auth.user.id}`) || 0)
    selectedKeyId.value = available.some(key => key.id === saved) ? saved : 0
    pendingKeyId.value = selectedKeyId.value || available[0]?.id || 0
    showKeyPicker.value = !selectedKeyId.value

    const savedImage = Number(localStorage.getItem(`sub2api:canvas-image-key:${auth.user.id}`) || 0)
    imageKeyId.value = available.some(key => key.id === savedImage) ? savedImage : 0

    const savedText = Number(localStorage.getItem(`sub2api:canvas-text-key:${auth.user.id}`) || 0)
    textKeyId.value = available.some(key => key.id === savedText) ? savedText : 0
  } catch { if (requestVersion === version) error.value = '无法读取画布设置或可用密钥，请检查登录状态后重试。' }
  finally { if (requestVersion === version) loading.value = false }
}
watch(() => auth.user?.id, (userId, previousUserId) => {
  if (userId === previousUserId) return
  invalidateFrame()
  closeKeyPicker()
  keys.value = []
  selectedKeyId.value = 0
  imageKeyId.value = 0
  textKeyId.value = 0
  pendingKeyId.value = 0
  if (userId) void initialize()
})
watch(() => auth.user?.role, (role, previousRole) => {
  if (previousRole && role !== previousRole) { invalidateFrame(); closeKeyPicker() }
})
watch(() => canvas.settings.enabled, enabled => { if (!enabled) { invalidateFrame(); closeKeyPicker() } })
function refreshSettings() { if (auth.user?.id) void canvas.fetchSettings().catch(() => {}) }
onMounted(() => {
  window.addEventListener('message', onMessage)
  window.addEventListener('focus', refreshSettings)
  void initialize()
})
onUnmounted(() => {
  version++
  pickerVersion++
  window.removeEventListener('message', onMessage)
  window.removeEventListener('focus', refreshSettings)
})
</script>
