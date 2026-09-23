<template>
  <section class="space-y-5" aria-label="鹈鹕测试与能力展示">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">鹈鹕测试 · 能力展示</h1><p class="mt-2 text-sm text-gray-500">查看你可访问账号的公开作品，点击图像可放大查看。</p></div>
      <button class="btn btn-secondary" :disabled="loading" @click="load()">刷新作品</button>
    </div>
    <p v-if="loading && !accounts.length" class="rounded-xl border border-dashed border-gray-200 py-16 text-center text-sm text-gray-500 dark:border-dark-700" role="status">正在加载公开作品…</p>
    <div v-else-if="error" class="rounded-xl bg-red-50 p-5 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">{{ error }} <button class="ml-2 underline" @click="load()">重试</button></div>
    <div v-else-if="!accounts.length" class="rounded-xl border border-dashed border-gray-200 p-10 text-center dark:border-dark-700"><p class="text-sm font-medium text-gray-700 dark:text-gray-300">暂无公开作品</p><p class="mt-2 text-sm text-gray-500">管理员公开结果后，你有权限访问的账号作品会显示在这里。</p></div>
    <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <article v-for="account in accounts" :key="account.account_id" class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <h2 class="mb-4 text-sm font-semibold">{{ account.platform }} · {{ account.account_type }} #{{ account.account_id }}</h2>
        <div v-for="record in account.tests" :key="record.id" class="mt-3 space-y-3 border-t border-gray-100 pt-3 text-sm dark:border-dark-700">
          <div class="flex items-start justify-between gap-3"><div><p>{{ testName(record.test_type) }}</p><p class="mt-1 text-xs text-gray-500">{{ testTime(record.created_at) }}</p></div><button class="text-xs text-primary-600" @click="detailId = record.id">{{ record.test_type === 'pelican' ? '查看原图' : '查看结果' }}</button></div>
          <button v-if="record.test_type === 'pelican'" class="aspect-[4/3] w-full overflow-hidden rounded-xl" aria-label="查看生成的鹈鹕图像" @click="detailId = record.id"><TestGeneratedImage :source="record.result_image" :record-id="record.id" public-view /></button>
          <template v-else><TestStatusBadge :status="record.status" /><TestAssessment :assessment="record.evaluation" :status="record.status" compact public-view /></template>
        </div>
        <button class="mt-4 text-xs text-gray-500 hover:text-primary-600" @click="openHistory(account.account_id)">查看历史作品与结果</button>
      </article>
    </div>
    <Pagination v-if="total > 12" :page="page" :page-size="12" :total="total" :show-page-size-selector="false" @update:page="changePage" />
    <BaseDialog :show="historyAccount !== null" :title="`账号 #${historyAccount} · 历史作品与结果`" width="wide" @close="historyAccount = null">
      <p v-if="historyLoading && !history.length" class="py-8 text-center text-sm text-gray-500" role="status">正在加载历史作品…</p>
      <p v-else-if="historyError" class="text-sm text-red-600" role="alert">{{ historyError }} <button class="ml-2 underline" @click="loadHistory(historyPage)">重试</button></p>
      <div v-else class="grid gap-4 sm:grid-cols-2">
        <article v-for="record in history" :key="record.id" class="space-y-3 rounded-xl border border-gray-100 p-3 text-sm dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-3"><div><p>{{ testName(record.test_type) }}</p><p class="mt-1 text-xs text-gray-500">{{ testTime(record.created_at) }}</p></div><button class="text-primary-600" @click="historyAccount = null; detailId = record.id">{{ record.test_type === 'pelican' ? '查看原图' : '查看结果' }}</button></div>
          <button v-if="record.test_type === 'pelican'" class="aspect-[4/3] w-full overflow-hidden rounded-xl" aria-label="查看历史鹈鹕图像" @click="historyAccount = null; detailId = record.id"><TestGeneratedImage :source="record.result_image" :record-id="record.id" public-view /></button>
          <template v-else><TestStatusBadge :status="record.status" /><TestAssessment :assessment="record.evaluation" :status="record.status" compact public-view /></template>
        </article>
      </div>
      <p v-if="!history.length && !historyError && !historyLoading" class="py-8 text-center text-sm text-gray-500">暂无公开历史作品</p>
      <Pagination v-if="historyTotal > 12" :page="historyPage" :page-size="12" :total="historyTotal" :show-page-size-selector="false" @update:page="loadHistory" />
    </BaseDialog>
    <TestDetailDialog :record-id="detailId" public-view @close="detailId = null" />
  </section>
</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { intelligentTestsAPI, type PublicTestAccount, type PublicTestRecord } from '@/api/intelligentTests'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import TestDetailDialog from '@/components/admin/intelligent-tests/TestDetailDialog.vue'
import TestStatusBadge from '@/components/admin/intelligent-tests/TestStatusBadge.vue'
import TestAssessment from '@/components/admin/intelligent-tests/TestAssessment.vue'
import TestGeneratedImage from '@/components/admin/intelligent-tests/TestGeneratedImage.vue'
import { testName, testTime } from '@/components/admin/intelligent-tests/display'
const accounts = ref<PublicTestAccount[]>([]), page = ref(1), total = ref(0), detailId = ref<number | null>(null)
const loading = ref(true), error = ref('')
const historyAccount = ref<number | null>(null), history = ref<PublicTestRecord[]>([]), historyPage = ref(1), historyTotal = ref(0), historyError = ref(''), historyLoading = ref(false)
let timer: ReturnType<typeof setInterval>, disposed = false, version = 0, inFlight = false, historyVersion = 0, historyInFlight = false
async function load(quiet = false) {
  if (quiet && inFlight) return
  const current = ++version
  inFlight = true
  if (!quiet) loading.value = true
  try {
    const result = await intelligentTestsAPI.publicAccounts(page.value)
    if (disposed || current !== version) return
    if (!result.items.length && page.value > 1) { page.value = 1; inFlight = false; void load(); return }
    accounts.value = result.items; total.value = result.total; error.value = ''
    // Visibility changes remove stale public works and open dialogs immediately.
    if (!accounts.value.length) { detailId.value = null; historyAccount.value = null }
  } catch (err) {
    if (!disposed && current === version) {
      accounts.value = []; total.value = 0; detailId.value = null; historyAccount.value = null
      error.value = extractApiErrorMessage(err, '公开作品暂时无法加载，请稍后重试。')
    }
  } finally { if (current === version) { inFlight = false; loading.value = false } }
}
function changePage(value: number) { page.value = value; void load() }
function openHistory(id: number) { historyAccount.value = id; history.value = []; historyTotal.value = 0; void loadHistory(1) }
async function loadHistory(value: number, quiet = false) {
  const id = historyAccount.value
  if (id === null || (quiet && historyInFlight)) return
  const current = ++historyVersion
  historyInFlight = true; historyPage.value = value
  if (!quiet) historyLoading.value = true
  historyError.value = ''
  try {
    const data = await intelligentTestsAPI.publicTests(id, value)
    if (id !== historyAccount.value || disposed || current !== historyVersion) return
    history.value = data.items; historyPage.value = value; historyTotal.value = data.total
  } catch {
    if (id === historyAccount.value && !disposed && current === historyVersion) { history.value = []; historyTotal.value = 0; historyError.value = '历史作品暂不可用或已停止公开。' }
  } finally { if (current === historyVersion) { historyInFlight = false; historyLoading.value = false } }
}
onMounted(() => { void load(); timer = setInterval(() => { if (!document.hidden) { void load(true); if (historyAccount.value !== null) void loadHistory(historyPage.value, true) } }, 5000) })
onUnmounted(() => { disposed = true; version++; historyVersion++; clearInterval(timer) })
</script>
