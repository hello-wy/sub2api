<template>
  <div class="space-y-5">
    <div class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 text-sm leading-relaxed text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300">启用“用户可查看”后，有对应分组访问权限的用户可从侧栏“鹈鹕测试”查看公开作品。关闭后立即停止公开该类结果。定时测试使用下方保存的模型、思考强度和题目。</div>
    <div role="tablist" aria-label="测试类型设置" class="flex flex-wrap gap-3">
      <button v-for="setting in drafts" :key="setting.test_type" type="button" role="tab" :aria-selected="activeType === setting.test_type" :aria-controls="`settings-${setting.test_type}`" :class="activeType === setting.test_type ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'border-gray-200 bg-white text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300'" class="rounded-xl border px-5 py-3 text-left" @click="activeType = setting.test_type"><span class="block text-sm font-semibold">{{ setting.name || testName(setting.test_type) }}</span><span class="mt-1 block text-xs">{{ setting.enabled ? '已启用' : '未启用' }} · {{ setting.schedule.enabled ? '定时运行' : '手动运行' }}</span></button>
    </div>
    <form v-for="setting in drafts" v-show="activeType === setting.test_type" :id="`settings-${setting.test_type}`" :key="setting.test_type" role="tabpanel" :aria-label="setting.name || testName(setting.test_type)" class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" @submit.prevent="save(setting)">
      <fieldset :disabled="saving === setting.test_type" @input="markDirty(setting.test_type)" @change="markDirty(setting.test_type)">
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-100 pb-4 dark:border-dark-700">
        <div><h2 class="font-semibold">{{ setting.name || testName(setting.test_type) }}</h2><p class="mt-1 text-xs text-gray-500">{{ setting.test_type === 'pelican' ? '生成作品展示' : '独立的题目、模型和评分规则' }}</p></div>
        <div class="flex flex-wrap gap-5 text-sm">
          <label class="flex items-center gap-2"><input v-model="setting.enabled" type="checkbox" class="rounded text-primary-600" /> 管理员可用</label>
          <label class="flex items-center gap-2"><input v-model="setting.user_visible" type="checkbox" class="rounded text-primary-600" /> 用户可查看</label>
        </div>
      </div>
      <h3 class="mt-5 text-sm font-semibold">模型与请求</h3>
      <div class="mt-3 grid gap-4 sm:grid-cols-2">
        <label class="block text-sm">模型<input v-model.trim="setting.config.model" class="input mt-2" :list="`intelligent-models-${setting.test_type}`" placeholder="留空使用账号默认模型" maxlength="200" />
          <datalist :id="`intelligent-models-${setting.test_type}`"><option v-for="model in modelSuggestions" :key="model" :value="model" /></datalist>
          <span class="mt-1 block text-xs text-gray-500">OpenAI 默认使用 gpt-5.6-sol；也可填写当前账号支持的模型 ID。</span>
        </label>
        <label class="block text-sm">超时（秒）<input v-model.number="setting.config.timeout_seconds" type="number" class="input mt-2" min="30" max="600" required /></label>
        <label class="block text-sm sm:col-span-2">思考强度
          <select v-model="setting.config.reasoning_effort" class="input mt-2" :aria-label="`${testName(setting.test_type)}思考强度`"><option v-for="option in reasoningOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select>
          <span class="mt-1 block text-xs text-gray-500">模型默认保留账号默认行为。指定强度需要所选模型和接口支持；不支持时会提示原因。</span>
        </label>
        <div class="sm:col-span-2 flex flex-wrap gap-2"><button v-for="model in modelSuggestions" :key="model" type="button" class="rounded-full border px-3 py-1.5 text-xs dark:border-dark-600" :class="setting.config.model === model ? 'border-primary-400 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'border-gray-200 text-gray-500'" @click="setting.config.model = model; markDirty(setting.test_type)">{{ model }}</button></div>
        <h3 class="mt-2 text-sm font-semibold sm:col-span-2">{{ setting.test_type === 'pelican' ? '绘图要求' : '题目与判断规则' }}</h3>
        <label class="block text-sm sm:col-span-2">测试题目<textarea v-model="setting.config.prompt" class="input mt-2 min-h-32" maxlength="16000" required /></label>
        <label v-if="setting.test_type !== 'pelican'" class="block text-sm">评分规则<select v-model="setting.config.evaluator" class="input mt-2"><option value="svg_structure">SVG 结构校验</option><option value="exact_answer">标准答案校验</option></select></label>
        <template v-if="setting.test_type !== 'pelican' && setting.config.evaluator === 'exact_answer'">
          <label class="block text-sm">标准答案<input v-model="setting.config.expected_answer" class="input mt-2" maxlength="200" required /></label>
          <label class="block text-sm">答案类型<select v-model="setting.config.answer_type" class="input mt-2"><option value="auto">自动识别数值或文本</option><option value="number">数值等价比较</option><option value="text">文本比较</option></select></label>
          <label class="block text-sm">单位规则<select v-model="setting.config.answer_unit_mode" class="input mt-2"><option value="none">不接受单位</option><option value="configured">只接受指定单位</option><option value="legacy">沿用旧题兼容规则</option></select></label>
          <label v-if="setting.config.answer_unit_mode !== 'none'" class="block text-sm">允许的数值单位<input v-model.trim="setting.config.answer_unit" class="input mt-2" maxlength="32" :required="setting.config.answer_unit_mode === 'configured'" placeholder="例如：颗（也接受颗糖）" /><span v-if="setting.config.answer_unit_mode === 'legacy'" class="mt-1 block text-xs text-gray-500">旧糖果题留空时兼容“颗”；选择“不接受单位”可明确关闭。</span></label>
          <label class="block text-sm">独立格式要求<select v-model="setting.config.answer_format" class="input mt-2"><option value="answer_line">唯一末行 ANSWER: 答案</option><option value="free_text">不要求固定格式</option></select></label>
        </template>
      </div>
      <p class="mt-3 text-xs leading-relaxed text-gray-500">{{ setting.test_type === 'pelican' ? '展示生成的图像，支持 HTML 答复中的 SVG。不显示成功、失败或评分。' : '答案与格式分别判断。12、12.0、全角数字及允许单位可数值等价；Markdown 和格式差异不扣答案分。多个冲突答案显示无法判定。仅检查最终答案，不验证完整推导。修改题目时请同步标准答案与单位。' }}</p>
      <section class="mt-5 rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <label class="flex items-center gap-2 text-sm font-medium"><input v-model="setting.schedule.enabled" type="checkbox" class="rounded text-primary-600" :aria-label="`${testName(setting.test_type)}启用定时测试`" />启用定时测试</label>
        <p class="mt-2 text-xs text-gray-500">按固定间隔自动生成新结果，浏览器关闭后仍会执行。启用后首次在一个间隔后运行；会产生上游用量。</p>
        <div class="mt-3 grid gap-4 sm:grid-cols-2">
          <label class="text-sm">间隔（分钟）<input v-model.number="setting.schedule.interval_minutes" type="number" min="5" max="10080" step="1" required class="input mt-2" :aria-label="`${testName(setting.test_type)}定时间隔`" /><span class="mt-1 block text-xs text-gray-500">5 分钟至 7 天</span></label>
          <div class="space-y-2 pt-1 text-xs text-gray-500"><p>下次运行：{{ setting.schedule.enabled && setting.enabled ? (setting.schedule.next_run_at ? testTime(setting.schedule.next_run_at) : '保存后安排') : '未启用' }}</p><p>上次运行：{{ setting.schedule.last_run_at ? testTime(setting.schedule.last_run_at) : '尚未运行' }}</p><p v-if="setting.schedule.enabled && !setting.enabled" class="text-amber-700">需同时启用“管理员可用”才能运行定时任务。</p></div>
        </div>
        <div class="mt-4 flex items-center justify-between gap-2"><p class="text-sm">测试账号 <span class="text-xs text-gray-500">已选 {{ setting.schedule.account_ids.length }} / 100</span></p><button type="button" class="text-xs text-primary-600" @click="setting.schedule.account_ids = []; markDirty(setting.test_type)">清空选择</button></div>
        <div v-if="setting.schedule.account_ids.length" class="mt-2 flex flex-wrap gap-2"><button v-for="id in setting.schedule.account_ids" :key="id" type="button" class="rounded-full bg-primary-50 px-3 py-1 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300" :aria-label="`移除定时账号 ${id}`" @click="removeAccount(setting, id)">{{ accountNames[id] || `#${id}` }} ×</button></div>
        <div class="mt-2 flex gap-2"><input v-model.trim="accountSearch" class="input" placeholder="搜索账号名称、备注或编号" aria-label="搜索定时测试账号" @input.stop @change.stop @keydown.enter.prevent="loadAccounts(1)" /><button type="button" class="btn btn-secondary" :disabled="accountsLoading" @click="loadAccounts(1)">搜索</button></div>
        <p v-if="accountsLoading" class="py-4 text-sm text-gray-500">正在加载可选账号…</p>
        <p v-else-if="accountsError" class="mt-3 text-sm text-red-600" role="alert">{{ accountsError }} <button type="button" class="underline" @click="loadAccounts(accountPage)">重试</button></p>
        <div v-else class="mt-2 max-h-52 space-y-1 overflow-y-auto rounded-lg bg-gray-50 p-2 dark:bg-dark-900">
          <label v-for="account in accounts" :key="account.id" class="flex items-start gap-2 rounded px-2 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-800"><input v-model="setting.schedule.account_ids" type="checkbox" :value="account.id" class="mt-1 rounded text-primary-600" :disabled="setting.schedule.account_ids.length >= 100 && !setting.schedule.account_ids.includes(account.id)" /><span>{{ account.name || `账号 #${account.id}` }}<span class="mt-0.5 block text-xs text-gray-500">#{{ account.id }} · {{ account.platform }} · {{ account.type }}{{ account.status !== 'active' ? ' · 当前不可用' : '' }}</span></span></label>
          <p v-if="!accounts.length" class="p-3 text-sm text-gray-500">没有符合条件的账号</p>
        </div>
        <div v-if="accountTotal > 50" class="mt-2 flex items-center justify-end gap-3 text-xs text-gray-500"><button type="button" class="text-primary-600 disabled:opacity-40" :disabled="accountsLoading || accountPage === 1" @click="loadAccounts(accountPage - 1, true)">上一页</button><span>第 {{ accountPage }} 页 / {{ Math.ceil(accountTotal / 50) }} 页</span><button type="button" class="text-primary-600 disabled:opacity-40" :disabled="accountsLoading || accountPage * 50 >= accountTotal" @click="loadAccounts(accountPage + 1, true)">下一页</button></div>
        <p v-if="setting.schedule.last_error" class="mt-3 text-xs text-amber-700">最近调度提示：{{ setting.schedule.last_error }}</p>
      </section>
      <details class="mt-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
        <summary class="cursor-pointer text-sm">{{ setting.test_type === 'pelican' ? '粘贴答复预览图像（不调用模型）' : '粘贴答复试判（使用当前草稿，不调用模型）' }}</summary>
        <textarea v-model="trialOutputs[setting.test_type]" class="input mt-3 min-h-24" maxlength="131072" placeholder="粘贴模型答复或 SVG" />
        <button type="button" class="btn btn-secondary mt-2" :disabled="!!previewing || !trialOutputs[setting.test_type]" @click="preview(setting)">{{ previewing === setting.test_type ? '正在处理…' : setting.test_type === 'pelican' ? '预览图像' : '试判当前答复' }}</button>
        <div v-if="previews[setting.test_type]" class="mt-3 space-y-3">
          <TestAssessment v-if="setting.test_type !== 'pelican'" :assessment="previews[setting.test_type]?.evaluation" :status="previews[setting.test_type]?.status" />
          <TestGeneratedImage v-if="setting.test_type === 'pelican' || previews[setting.test_type]?.result_image" :source="previews[setting.test_type]?.result_image" />
        </div>
      </details>
      <p v-if="errors[setting.test_type]" class="mt-3 text-sm text-red-600" role="alert">{{ errors[setting.test_type] }}</p>
      <div class="mt-4 flex items-center justify-end gap-3"><span v-if="saved === setting.test_type" class="text-sm text-emerald-600">已保存</span><button class="btn btn-primary" :disabled="!!saving">{{ saving === setting.test_type ? '保存中…' : '保存设置' }}</button></div>
      </fieldset>
    </form>
  </div>
</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { intelligentTestsAPI, type TestSetting, type TestSchedule, type TestRecord } from '@/api/intelligentTests'
import { list as listAccounts } from '@/api/admin/accounts'
import type { AccountListItem } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { testName, testTime, reasoningOptions } from './display'
import TestAssessment from './TestAssessment.vue'
import TestGeneratedImage from './TestGeneratedImage.vue'
import { DEFAULT_IMPORT_MODELS } from '@/utils/accountImportPreview'
const props = defineProps<{ settings: TestSetting[] }>()
const emit = defineEmits<{ saved: [setting: TestSetting] }>()
type SettingDraft = TestSetting & { schedule: TestSchedule }
const drafts = ref<SettingDraft[]>([]), saving = ref(''), saved = ref(''), errors = ref<Record<string, string>>({})
const activeType = ref('')
const accounts = ref<AccountListItem[]>([]), accountsLoading = ref(false), accountsError = ref(''), accountSearch = ref('')
const accountPage = ref(1), accountTotal = ref(0), accountNames = ref<Record<number, string>>({})
let appliedAccountSearch = ''
let accountController: AbortController | undefined
function removeAccount(setting: SettingDraft, id: number) { setting.schedule.account_ids = setting.schedule.account_ids.filter(value => value !== id); markDirty(setting.test_type) }
async function loadAccounts(page = 1, keepSearch = false) {
  accountController?.abort(); const controller = new AbortController(); accountController = controller
  accountsLoading.value = true; accountsError.value = ''
  if (!keepSearch) appliedAccountSearch = accountSearch.value
  try {
    const data = await listAccounts(page, 50, { lite: 'true', search: appliedAccountSearch }, { signal: controller.signal })
    if (controller.signal.aborted) return
    accounts.value = data.items; accountPage.value = page; accountTotal.value = data.total
    for (const account of data.items) accountNames.value[account.id] = `${account.name || '账号'} #${account.id}`
  } catch (error) { if (!controller.signal.aborted) accountsError.value = extractApiErrorMessage(error, '账号列表暂不可用') }
  finally { if (!controller.signal.aborted) accountsLoading.value = false }
}
onMounted(() => loadAccounts())
onUnmounted(() => accountController?.abort())
const dirty = new Set<string>()
const trialOutputs = ref<Record<string, string>>({}), previews = ref<Record<string, TestRecord | undefined>>({}), previewing = ref('')
function markDirty(type: string) { dirty.add(type); saved.value = ''; previews.value[type] = undefined }
// Suggestions remain editable; explicit choices are preserved by the runner.
const modelSuggestions = DEFAULT_IMPORT_MODELS
watch(() => props.settings, value => {
  if (!value.some(item => item.test_type === activeType.value)) activeType.value = value[0]?.test_type || ''
  drafts.value = value.map(item => {
    const existing = drafts.value.find(draft => draft.test_type === item.test_type)
    if (existing && dirty.has(item.test_type)) return existing
    return { ...item, config: { answer_type: 'auto', answer_format: 'answer_line', answer_unit_mode: item.config.answer_unit ? 'configured' : 'legacy', reasoning_effort: '', ...item.config }, schedule: { enabled: false, interval_minutes: 60, ...item.schedule, account_ids: [...(item.schedule?.account_ids ?? [])] } }
  })
}, { immediate: true, deep: true })
async function preview(setting: TestSetting) {
  if (previewing.value) return
  const type = setting.test_type, output = trialOutputs.value[type] || '', config = { ...setting.config }
  previewing.value = type; errors.value[type] = ''
  try {
    const result = await intelligentTestsAPI.previewEvaluation(output, config)
    if (output === trialOutputs.value[type] && JSON.stringify(config) === JSON.stringify(setting.config)) previews.value[type] = result
  } catch (err) { errors.value[type] = extractApiErrorMessage(err, '试判失败') }
  finally { previewing.value = '' }
}
async function save(setting: SettingDraft) {
  if (saving.value) return
  if (setting.schedule.enabled && !setting.schedule.account_ids.length) { errors.value[setting.test_type] = '请至少选择一个定时测试账号'; return }
  if (!Number.isInteger(setting.schedule.interval_minutes) || setting.schedule.interval_minutes < 5 || setting.schedule.interval_minutes > 10080) { errors.value[setting.test_type] = '定时间隔应为 5–10080 之间的整数分钟'; return }
  const snapshot: TestSetting = { ...setting, config: { ...setting.config }, schedule: { ...setting.schedule, account_ids: [...setting.schedule.account_ids] } }
  saving.value = setting.test_type; saved.value = ''; errors.value[setting.test_type] = ''
  try { const result = await intelligentTestsAPI.saveSetting(snapshot); dirty.delete(snapshot.test_type); saved.value = snapshot.test_type; emit('saved', result) }
  catch (error) { errors.value[setting.test_type] = extractApiErrorMessage(error, '设置保存失败') }
  finally { saving.value = '' }
}
</script>
