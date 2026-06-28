<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="card p-6">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="space-y-2">
            <div class="flex items-center gap-3">
              <div class="rounded-xl bg-primary-100 p-2 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
                <Icon name="calendar" size="lg" />
              </div>
              <div class="min-w-0">
                <h2 class="text-xl font-semibold text-gray-900 dark:text-white">每日签到</h2>
                <p class="text-sm text-gray-500 dark:text-dark-400">完成每日签到并领取当日奖励。</p>
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
              <span>{{ formatDateText(status?.today_date) }}</span>
              <span v-if="status?.timezone" class="rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                {{ status.timezone }}
              </span>
            </div>
          </div>

          <div class="flex flex-wrap gap-3">
            <button
              class="btn btn-primary"
              :disabled="alreadyCheckedIn || checkingIn || loading"
              @click="handleCheckin"
            >
              <Icon v-if="checkingIn" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else-if="alreadyCheckedIn" name="checkCircle" size="sm" />
              <Icon v-else name="sparkles" size="sm" />
              <span>{{ checkingIn ? '正在签到...' : alreadyCheckedIn ? '今日已签到' : '立即签到' }}</span>
            </button>
          </div>
        </div>
      </div>

      <BaseDialog
        :show="showQQBindDialog"
        title="绑定 QQ 后可签到"
        width="narrow"
        @close="showQQBindDialog = false"
      >
        <div class="space-y-4">
          <div class="rounded-2xl bg-amber-50 p-4 text-sm text-amber-800 ring-1 ring-amber-200 dark:bg-amber-900/20 dark:text-amber-200 dark:ring-amber-900/40">
            当前账号还未绑定 QQ。请前往 QQ 群完成平台账号绑定后，再返回每日签到。
          </div>
          <p v-if="contactInfo" class="text-sm text-gray-600 dark:text-dark-300">
            {{ contactInfo }}
          </p>
        </div>
        <template #footer>
          <button class="btn btn-secondary" @click="showQQBindDialog = false">关闭</button>
        </template>
      </BaseDialog>

      <div v-if="loading" class="flex justify-center py-10">
        <LoadingSpinner size="lg" />
      </div>

      <template v-else>
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">今日可领</p>
            <p class="mt-2 text-3xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formatRewardRange(status?.today_reward_min ?? status?.today_reward ?? 0, status?.today_reward_max ?? status?.today_reward ?? 0) }}
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              基础奖励 {{ formatRewardRange(status?.base_reward_min ?? status?.base_reward ?? 0, status?.base_reward_max ?? status?.base_reward ?? 0) }}
              <span v-if="(status?.extra_reward ?? 0) > 0">
                · 额外奖励 {{ formatDollar(status?.extra_reward ?? 0) }}
              </span>
            </p>
          </div>

          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">连续签到</p>
            <p class="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
              {{ status?.current_streak ?? 0 }}<span class="ml-1 text-base font-medium text-gray-500 dark:text-dark-400">天</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              今天：{{ alreadyCheckedIn ? '已签到' : '未签到' }}
            </p>
          </div>

          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">本月签到</p>
            <p class="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
              {{ status?.month_checkins ?? 0 }}<span class="ml-1 text-base font-medium text-gray-500 dark:text-dark-400">天</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ status?.timezone || '' }}
            </p>
          </div>

          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">累计奖励</p>
            <p class="mt-2 text-3xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatDollar(status?.total_reward ?? 0) }}
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              合计可领 {{ formatRewardRange(status?.today_reward_min ?? status?.today_reward ?? 0, status?.today_reward_max ?? status?.today_reward ?? 0) }}
            </p>
          </div>
        </div>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,7fr)_minmax(0,5fr)]">
          <div class="card p-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">连续签到奖励规则</h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  每 30 天为一个周期，达到对应天数时发放额外奖励。
                </p>
              </div>
              <div class="shrink-0 rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                每 30 天为一个周期
              </div>
            </div>

            <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div
                v-for="rule in rewardRules"
                :key="rule.day_count"
                class="flex min-h-[112px] min-w-0 flex-col justify-between rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-4 dark:border-emerald-900/40 dark:bg-emerald-900/20"
              >
                <div class="space-y-1">
                  <div class="whitespace-nowrap text-sm font-semibold leading-none text-emerald-800 dark:text-emerald-200">
                    连续 {{ rule.day_count }} 天
                  </div>
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-xs font-medium uppercase tracking-wide text-emerald-500/80 dark:text-emerald-300/70">
                    Extra
                  </span>
                  <div class="shrink-0 whitespace-nowrap rounded-full bg-white/80 px-3 py-1 text-xs font-semibold text-emerald-700 shadow-sm ring-1 ring-emerald-200 dark:bg-dark-950/30 dark:text-emerald-200 dark:ring-emerald-900/50">
                    +{{ formatDollar(rule.extra_reward) }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="card p-6">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">近 7 天签到日历</h3>
            <div class="mt-4 grid grid-cols-7 gap-2">
              <div
                v-for="day in calendarDays"
                :key="day.date"
                class="group min-h-[110px] min-w-0 rounded-2xl border px-2 py-2 transition-colors"
                :class="day.checked_in
                  ? 'border-emerald-200 bg-emerald-50 dark:border-emerald-900/40 dark:bg-emerald-900/20'
                  : day.is_today
                    ? 'border-primary-200 bg-primary-50 dark:border-primary-900/40 dark:bg-primary-900/20'
                    : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900'"
              >
                <div class="flex items-center justify-between gap-1">
                  <div class="text-sm font-semibold leading-none tracking-tight text-gray-900 dark:text-white">
                    {{ formatCalendarDay(day.date) }}
                  </div>
                  <span
                    v-if="day.is_today"
                    class="whitespace-nowrap rounded-full bg-primary-100 px-1.5 py-0.5 text-[10px] font-medium text-primary-700 dark:bg-primary-900/40 dark:text-primary-200"
                  >
                    今天
                  </span>
                </div>
                <div class="mt-3 flex flex-col items-center gap-1.5 text-center">
                  <span
                    class="flex h-7 w-7 items-center justify-center rounded-full text-[10px] font-semibold"
                    :class="day.checked_in
                      ? 'bg-emerald-500/10 text-emerald-700 ring-1 ring-inset ring-emerald-300 dark:bg-emerald-500/15 dark:text-emerald-300 dark:ring-emerald-500/40'
                      : day.is_today
                        ? 'bg-primary-500/10 text-primary-700 ring-1 ring-inset ring-primary-300 dark:bg-primary-500/15 dark:text-primary-300 dark:ring-primary-500/40'
                        : 'bg-gray-50 text-gray-300 ring-1 ring-inset ring-gray-200 dark:bg-dark-800 dark:text-dark-500 dark:ring-dark-700'"
                  >
                    {{ day.checked_in ? '✓' : day.is_today ? '今' : '·' }}
                  </span>
                  <span class="text-[11px] font-medium text-gray-400 dark:text-dark-500">
                    {{ formatCalendarWeekday(day.date) }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <transition name="fade">
          <div v-if="message" class="card border-emerald-200 bg-emerald-50 p-5 dark:border-emerald-900/40 dark:bg-emerald-900/20">
            <div class="flex items-start gap-3">
              <div class="rounded-xl bg-emerald-100 p-2 text-emerald-600 dark:bg-emerald-900/40 dark:text-emerald-300">
                <Icon name="checkCircle" size="md" />
              </div>
              <div>
                <p class="font-semibold text-emerald-800 dark:text-emerald-200">{{ message }}</p>
                <p class="mt-1 text-sm text-emerald-700 dark:text-emerald-300">
                  +{{ formatDollar(lastClaim?.total_reward ?? 0) }}
                  <span v-if="lastClaim"> · {{ lastClaim.current_streak }}天</span>
                </p>
              </div>
            </div>
          </div>
        </transition>

        <div class="card p-6">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">签到明细</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">默认展示最近 7 天记录</p>
            </div>
            <button class="btn btn-secondary" @click="toggleHistory">
              {{ showAllHistory ? '收起' : '查看全部' }}
            </button>
          </div>

          <div v-if="history.length === 0" class="mt-5 rounded-2xl border border-dashed border-gray-300 px-6 py-12 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            最近 7 天暂无签到记录
          </div>

          <div v-else class="mt-5 overflow-x-auto">
            <table class="w-full min-w-[760px] text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-3 py-3 font-medium">日期</th>
                  <th class="px-3 py-3 font-medium">连续天数</th>
                  <th class="px-3 py-3 font-medium">基础奖励</th>
                  <th class="px-3 py-3 font-medium">额外奖励</th>
                  <th class="px-3 py-3 font-medium">合计奖励</th>
                  <th class="px-3 py-3 font-medium">状态</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in history"
                  :key="item.id"
                  class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                >
                  <td class="px-3 py-4 font-medium text-gray-900 dark:text-white">
                    {{ formatDateText(item.checkin_date) }}
                  </td>
                  <td class="px-3 py-4 text-gray-700 dark:text-dark-300">
                    {{ item.streak_days }}
                  </td>
                  <td class="px-3 py-4 text-gray-700 dark:text-dark-300">
                    {{ formatDollar(item.base_reward) }}
                  </td>
                  <td class="px-3 py-4 text-gray-700 dark:text-dark-300">
                    {{ formatDollar(item.extra_reward) }}
                  </td>
                  <td class="px-3 py-4 font-medium text-emerald-600 dark:text-emerald-400">
                    {{ formatDollar(item.total_reward) }}
                  </td>
                  <td class="px-3 py-4">
                    <span class="inline-flex items-center rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                      已签到
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { checkinAPI, type CheckinHistoryItem, type CheckinRewardRule, type CheckinStatusResponse } from '@/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateOnly } from '@/utils/format'

const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(true)
const checkingIn = ref(false)
const showQQBindDialog = ref(false)
const showAllHistory = ref(false)
const status = ref<CheckinStatusResponse | null>(null)
const history = ref<CheckinHistoryItem[]>([])
const lastClaim = ref<{ total_reward: number; current_streak: number; message: string } | null>(null)
const message = ref('')

const rewardRules = computed<CheckinRewardRule[]>(() => {
  const rules = status.value?.reward_rules?.length
    ? status.value.reward_rules
    : [
        { day_count: 3, extra_reward: 3 },
        { day_count: 7, extra_reward: 6 },
        { day_count: 14, extra_reward: 12 },
        { day_count: 30, extra_reward: 24 },
      ]
  return [...rules].sort((a, b) => a.day_count - b.day_count)
})

const qqBound = computed(() => status.value?.qq_bound ?? false)
const alreadyCheckedIn = computed(() => status.value?.already_checked_in ?? false)
const contactInfo = computed(() => appStore.contactInfo)

type CalendarDay = {
  date: string
  checked_in: boolean
  is_today: boolean
  reward?: number
}

const calendarDays = computed<CalendarDay[]>(() => {
  const days = new Map<string, { checked_in: boolean; reward?: number }>()
  for (const item of status.value?.recent_days ?? []) {
    days.set(item.date, {
      checked_in: item.checked_in,
      reward: item.reward
    })
  }
  for (const item of history.value) {
    days.set(item.checkin_date, {
      checked_in: true,
      reward: item.total_reward
    })
  }

  const baseDate = status.value?.today_date ? new Date(`${status.value.today_date}T00:00:00`) : new Date()
  const out: CalendarDay[] = []
  for (let i = 3; i >= -3; i -= 1) {
    const d = new Date(baseDate)
    d.setDate(baseDate.getDate() - i)
    const date = d.toISOString().slice(0, 10)
    const hit = days.get(date)
    out.push({
      date,
      checked_in: hit?.checked_in ?? false,
      is_today: i === 0,
      reward: hit?.reward
    })
  }
  return out
})

function formatDateText(date?: string): string {
  return date ? formatDateOnly(date) : '-'
}

function formatDollar(value: number): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function formatRewardRange(min: number, max: number): string {
  const normalizedMin = Number(min || 0)
  const normalizedMax = Number(max || normalizedMin)
  if (normalizedMin === normalizedMax) return formatDollar(normalizedMin)
  return `${formatDollar(normalizedMin)} - ${formatDollar(normalizedMax)}`
}

function formatCalendarDay(date: string): string {
  const parts = date.split('-')
  if (parts.length === 3) return `${Number(parts[2])}`
  return date
}

function formatCalendarWeekday(date: string): string {
  const d = new Date(`${date}T00:00:00`)
  const labels = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
  return labels[d.getDay()] || ''
}

async function loadStatus(): Promise<void> {
  try {
    status.value = await checkinAPI.getCheckinStatus()
    history.value = status.value.recent_history ?? []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '请求失败'))
  }
}

async function loadHistory(forceAll = false): Promise<void> {
  if (!forceAll) {
    history.value = status.value?.recent_history ?? []
    return
  }
  try {
    const resp = await checkinAPI.getCheckinHistory(1, 20)
    history.value = resp.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '请求失败'))
  }
}

async function refreshAll(): Promise<void> {
  loading.value = true
  try {
    await Promise.all([
      authStore.refreshUser().catch(() => undefined),
      loadStatus(),
    ])
    await loadHistory(showAllHistory.value)
  } finally {
    loading.value = false
  }
}

async function handleCheckin(): Promise<void> {
  if (checkingIn.value || alreadyCheckedIn.value) return
  if (!qqBound.value) {
    showQQBindDialog.value = true
    return
  }
  checkingIn.value = true
  message.value = ''
  try {
    const resp = await checkinAPI.checkin()
    lastClaim.value = {
      total_reward: resp.total_reward,
      current_streak: resp.current_streak,
      message: resp.message,
    }
    message.value = resp.message
    appStore.showSuccess(resp.message)
    await Promise.all([
      authStore.refreshUser().catch(() => undefined),
      loadStatus(),
    ])
    history.value = status.value?.recent_history ?? history.value
    if (showAllHistory.value) {
      await loadHistory(true)
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '请求失败'))
  } finally {
    checkingIn.value = false
  }
}

async function toggleHistory(): Promise<void> {
  showAllHistory.value = !showAllHistory.value
  await loadHistory(showAllHistory.value)
}

onMounted(() => {
  void refreshAll()
})
</script>
