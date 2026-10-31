<template>
  <AppLayout>
    <section class="flex h-[calc(100dvh-8rem)] min-h-0 flex-col gap-3">
      <p v-if="error" role="alert" class="text-sm text-red-500">{{ error }}</p>
      <button v-if="!loading && error" type="button" class="btn btn-secondary btn-sm self-start" @click="reconnect">重新连接</button>
      <p v-if="!loading && !canvas.settings.enabled" class="text-sm">管理员尚未开启无限画布。</p>
      <p v-else-if="!loading && !canvas.settings.url" class="text-sm">尚未配置画布地址，请在 SolidAPI 管理设置中填写画布地址。</p>
      <p v-else-if="!loading && !frameUrl" role="alert" class="text-sm text-red-500">画布地址无效，请在功能开关中检查画布地址。</p>
      <iframe v-if="auth.user && canvas.settings.enabled && frameUrl && !loading" :key="frameVersion" ref="frame" :src="frameUrl"
        title="无限画布" class="min-h-0 w-full flex-1 border-0" allow="clipboard-read; clipboard-write" allowfullscreen referrerpolicy="origin" @load="syncTheme" />
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { getCurrentUser } from '@/api/auth'
import { keysAPI } from '@/api/keys'
import { useCanvasStore } from '@/stores/canvas'
import { CANVAS_PROTOCOL, buildCanvasUrl, canvasKeyAvailable, trustedCanvasMessage } from '@/utils/canvas-bridge'
import { detectTheme } from '@/utils/embedded-url'

const auth = useAuthStore()
const canvas = useCanvasStore()
const router = useRouter()
const { locale } = useI18n()
const frame = ref<HTMLIFrameElement | null>(null)
const loading = ref(true)
const error = ref('')
const frameVersion = ref(0)
let version = 0
let themeObserver: MutationObserver | null = null
const initialTheme = detectTheme()
const initialLocale = locale.value
const frameUrl = computed(() => {
  try { return canvas.settings.url ? buildCanvasUrl(canvas.settings.url, initialTheme, initialLocale) : '' }
  catch { return '' }
})

function syncTheme() {
  if (frameUrl.value) frame.value?.contentWindow?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'theme', theme: detectTheme() }, new URL(frameUrl.value).origin)
}

async function onMessage(event: MessageEvent) {
  const target = frame.value?.contentWindow || null
  if (!canvas.settings.enabled || !trustedCanvasMessage(event, target, frameUrl.value)) return
  const request = event.data
  if (request.type === 'reconnect') { void reconnect(); return }
  if (!['session', 'navigate', 'keys', 'key'].includes(request.type)) return
  const requestVersion = version
  const userId = auth.user?.id
  const current = () => requestVersion === version && userId === auth.user?.id && target === frame.value?.contentWindow && canvas.settings.enabled
  try {
    if (request.type === 'navigate') {
      const { data: user } = await getCurrentUser()
      if (requestVersion !== version || user.id !== auth.user?.id) return
      const paths = ['/keys', '/usage', ...(user.role === 'admin' ? ['/admin/accounts', '/admin/groups', '/admin/settings', '/admin/settings?tab=features', '/admin/users'] : [])]
      if (paths.includes(request.path)) void router.push(request.path)
      return
    }
    const [{ data: user }, settings] = await Promise.all([getCurrentUser(), canvas.fetchSettings()])
    if (!current()) return
    if (user.id !== userId || !['admin', 'user'].includes(user.role)) throw new Error('账号或权限已变化，请重新登录后连接画布。')
    if (!settings.enabled) throw new Error('无限画布已关闭，请联系管理员。')
    if (settings.url !== canvas.settings.url) throw new Error('画布地址已变化，请重新连接。')
    if (request.type === 'keys') {
      const keys = []
      let page = 1
      let pages = 1
      do {
        const result = await keysAPI.list(page)
        if (!current()) return
        keys.push(...result.items.filter(key => key.user_id === user.id).map(key => ({ id: key.id, name: key.name, groupName: key.group?.name || '', available: canvasKeyAvailable(key) })))
        pages = result.pages
        page++
      } while (page <= pages)
      target?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'keys', requestId: request.requestId, result: keys }, event.origin)
      return
    }
    if (request.type === 'key') {
      if (!Number.isSafeInteger(request.id) || request.id <= 0) throw new Error('请选择有效的 Key。')
      const key = await keysAPI.getById(request.id)
      if (!current()) return
      if (key.id !== request.id || key.user_id !== user.id || !canvasKeyAvailable(key) || !key.key) throw new Error('该 Key 不属于当前账号或已不可用，请重新选择。')
      target?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'key', requestId: request.requestId, result: { id: key.id, key: key.key } }, event.origin)
      return
    }
    error.value = ''
    target?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'session', requestId: request.requestId, theme: detectTheme(), session: { user: { id: user.id, role: user.role }, upstreamRetryCount: settings.upstream_retry_count } }, event.origin)
  } catch (cause) {
    if (!current()) return
    const status = (cause as { status?: number } | null)?.status
    const message = status === 401 ? '登录已失效，请重新登录后连接画布。' : status === 403 ? '当前账号无权执行此操作。' : cause instanceof Error ? cause.message : status ? `连接平台失败（HTTP ${status}），请重新连接。` : '无法读取平台数据，请检查网络后重试。'
    if (request.type === 'session') error.value = message
    target?.postMessage({ protocol: CANVAS_PROTOCOL, type: request.type, requestId: request.requestId, error: message }, event.origin)
  }
}

function invalidateFrame() {
  version++
  if (frameUrl.value) frame.value?.contentWindow?.postMessage({ protocol: CANVAS_PROTOCOL, type: 'invalidate' }, new URL(frameUrl.value).origin)
}
async function initialize() {
  const requestVersion = ++version
  loading.value = true
  error.value = ''
  try { await canvas.fetchSettings() }
  catch { if (requestVersion === version) error.value = '无法读取画布设置，请检查登录状态后重试。' }
  finally { if (requestVersion === version) loading.value = false }
}
async function reconnect() {
  invalidateFrame()
  frameVersion.value++
  await initialize()
}
watch(() => auth.user?.id, (userId, previousUserId) => {
  if (userId === previousUserId) return
  invalidateFrame()
  frameVersion.value++
  if (userId) void initialize()
})
watch(() => auth.user?.role, (role, previousRole) => {
  if (previousRole && role !== previousRole) void reconnect()
})
watch(() => canvas.settings.enabled, enabled => { if (!enabled && !loading.value) invalidateFrame() })
watch(() => canvas.settings.url, () => { if (!loading.value) { invalidateFrame(); frameVersion.value++ } })
function refreshSettings() { if (auth.user?.id) void canvas.fetchSettings().catch(() => {}) }
onMounted(() => {
  themeObserver = new MutationObserver(syncTheme)
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  window.addEventListener('message', onMessage)
  window.addEventListener('focus', refreshSettings)
  void initialize()
})
onUnmounted(() => {
  themeObserver?.disconnect()
  version++
  window.removeEventListener('message', onMessage)
  window.removeEventListener('focus', refreshSettings)
})
</script>
