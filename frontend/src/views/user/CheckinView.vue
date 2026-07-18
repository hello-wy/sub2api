<template>
  <AppLayout>
    <div class="checkin-shell relative -mx-4 -my-6 min-h-[calc(100vh-4rem)] overflow-hidden px-4 py-8 sm:-mx-6 lg:-mx-8 lg:px-8">
      <div class="relative mx-auto max-w-6xl space-y-5">
        <div class="checkin-panel overflow-hidden p-6 sm:p-8">
          <div class="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
            <div class="flex min-w-0 flex-col gap-5 sm:flex-row sm:items-center">
              <img
                :src="checkinCalendarIcon"
                alt=""
                aria-hidden="true"
                class="mx-auto h-24 w-24 shrink-0 object-contain sm:mx-0 sm:h-28 sm:w-28"
              />
              <div class="min-w-0 text-center sm:text-left">
                <h2 class="text-3xl font-bold tracking-normal text-slate-950 dark:text-white">每日签到</h2>
                <p class="mt-2 text-base text-slate-600 dark:text-dark-300">完成每日签到并领取当日奖励，连续签到可获得更多奖励！</p>
                <div class="mt-4 flex flex-wrap items-center justify-center gap-2 text-sm text-slate-600 dark:text-dark-300 sm:justify-start">
                  <span class="inline-flex items-center gap-2 rounded-full bg-slate-100/80 px-3 py-2 ring-1 ring-white/80 dark:bg-dark-900/70 dark:ring-dark-700">
                    <Icon name="calendar" size="sm" />
                    {{ formatDateText(status?.today_date) }}
                  </span>
                  <span v-if="status?.timezone" class="inline-flex items-center gap-2 rounded-full bg-slate-100/80 px-3 py-2 ring-1 ring-white/80 dark:bg-dark-900/70 dark:ring-dark-700">
                    <Icon name="globe" size="sm" />
                    {{ status.timezone }}
                  </span>
                </div>
              </div>
            </div>

            <div class="flex flex-col items-center gap-3 lg:items-end">
              <button
                class="inline-flex min-h-[58px] min-w-[190px] items-center justify-center gap-3 rounded-2xl bg-gradient-to-r from-primary-400 to-primary-600 px-8 text-base font-semibold text-white shadow-[0_18px_32px_rgba(22,119,255,0.28)] transition hover:-translate-y-0.5 hover:shadow-[0_22px_36px_rgba(22,119,255,0.34)] disabled:cursor-not-allowed disabled:opacity-70 disabled:hover:translate-y-0"
                :disabled="alreadyCheckedIn || checkingIn || loading"
                @click="handleCheckin"
              >
                <Icon v-if="checkingIn" name="refresh" size="md" class="animate-spin" />
                <Icon v-else-if="alreadyCheckedIn" name="checkCircle" size="md" />
                <Icon v-else name="sparkles" size="md" />
                <span>{{ checkingIn ? '正在签到...' : alreadyCheckedIn ? '今日已签到' : '立即签到' }}</span>
              </button>
              <span class="text-sm text-slate-500 dark:text-dark-400">签到成功即可领取奖励</span>
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
            <div class="flex items-center justify-between gap-3 rounded-xl bg-gray-50 px-4 py-3 text-sm text-gray-700 ring-1 ring-gray-200 dark:bg-dark-900 dark:text-dark-200 dark:ring-dark-700">
              <span class="text-gray-500 dark:text-dark-400">QQ群</span>
              <span class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ qqGroupNumber }}</span>
            </div>
          </div>
          <template #footer>
            <button
              class="btn btn-primary"
              type="button"
              @click="openQQGroupInvite"
            >
              <Icon name="externalLink" size="sm" />
              <span>加入 QQ 群</span>
            </button>
            <button class="btn btn-secondary" type="button" @click="copyQQGroupNumber">
              <Icon name="copy" size="sm" />
              <span>复制群号</span>
            </button>
            <button class="btn btn-secondary" @click="showQQBindDialog = false">关闭</button>
          </template>
        </BaseDialog>

        <div v-if="loading" class="checkin-panel flex justify-center py-14">
          <LoadingSpinner size="lg" />
        </div>

        <template v-else>
          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div
              v-for="card in statCards"
              :key="card.label"
              class="checkin-panel min-h-[124px] p-5"
            >
              <p class="text-sm font-medium text-slate-500 dark:text-dark-300">{{ card.label }}</p>
              <p class="mt-2 text-3xl font-bold tracking-normal text-slate-950 dark:text-white" :class="card.valueClass">
                {{ card.value }}<span v-if="card.unit" class="ml-1 text-base font-semibold text-slate-700 dark:text-dark-300">{{ card.unit }}</span>
              </p>
              <p class="mt-1 text-xs text-slate-500 dark:text-dark-400">{{ card.hint }}</p>
            </div>
          </div>

          <div class="grid gap-5 xl:grid-cols-[minmax(0,7fr)_minmax(0,5fr)]">
            <div class="checkin-panel p-5 sm:p-6">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="flex items-start gap-3">
                  <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-950/60 dark:text-primary-300 dark:ring-primary-900/40">
                    <Icon name="gift" size="md" />
                  </div>
                  <div>
                    <h3 class="text-lg font-bold tracking-normal text-slate-950 dark:text-white">连续签到奖励规则</h3>
                    <p class="mt-1 text-sm text-slate-500 dark:text-dark-400">
                      每 30 天为一个周期，达到对应天数时发放额外奖励。
                    </p>
                  </div>
                </div>
                <div class="shrink-0 rounded-full bg-slate-100/90 px-3 py-1.5 text-xs font-semibold text-slate-600 ring-1 ring-slate-200 dark:bg-dark-900/70 dark:text-dark-300 dark:ring-dark-700">
                  每 30 天为一个周期
                </div>
              </div>

              <div class="mt-5 grid gap-2.5 sm:grid-cols-2">
                <div
                  v-for="rule in rewardRules"
                  :key="rule.day_count"
                  class="reward-tile group flex min-h-[76px] min-w-0 items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3.5 transition-colors hover:border-primary-200 hover:bg-primary-50/55 dark:border-dark-700 dark:bg-dark-900/55 dark:hover:border-primary-800 dark:hover:bg-primary-950/25"
                >
                  <div class="min-w-0">
                    <div class="text-xs font-semibold text-slate-500 dark:text-dark-400">连续签到</div>
                    <div class="mt-1 text-lg font-bold leading-none text-slate-950 dark:text-white">
                      {{ rule.day_count }}<span class="ml-1 text-xs font-semibold text-slate-500 dark:text-dark-400">天</span>
                    </div>
                  </div>
                  <div class="shrink-0 text-right">
                    <div class="text-xs font-semibold text-slate-500 dark:text-dark-400">额外奖励</div>
                    <div class="mt-1 text-base font-bold leading-none text-primary-600 dark:text-primary-300">
                      +{{ formatDollar(rule.extra_reward) }}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="checkin-panel p-5 sm:p-6">
              <div class="flex items-start gap-3">
                <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-950/60 dark:text-primary-300 dark:ring-primary-900/40">
                  <Icon name="calendar" size="md" />
                </div>
                <h3 class="pt-1 text-lg font-bold tracking-normal text-slate-950 dark:text-white">近 7 天签到日历</h3>
              </div>
              <div class="calendar-scroll mt-4 pb-1">
                <div class="calendar-grid">
                  <div
                    v-for="day in calendarDays"
                    :key="day.date"
                    class="group relative flex h-[104px] min-w-0 flex-col items-center justify-between rounded-xl border px-1.5 pb-2 pt-4 text-center transition hover:-translate-y-0.5 sm:h-[116px] sm:rounded-2xl sm:px-2 sm:pb-3 sm:pt-5"
                    :class="day.checked_in
                      ? 'border-primary-200 bg-primary-50/90 text-primary-700 shadow-sm dark:border-primary-900/40 dark:bg-primary-900/20 dark:text-primary-300'
                      : day.is_today
                        ? 'border-primary-300 bg-primary-50 text-primary-700 shadow-sm dark:border-primary-400/90 dark:bg-primary-900/45 dark:text-primary-200 dark:shadow-[inset_0_0_0_1px_rgba(145,202,255,0.18)]'
                        : 'border-slate-200 bg-white/70 text-slate-500 dark:border-dark-700 dark:bg-dark-900/60 dark:text-dark-400'"
                  >
                    <span
                      v-if="day.is_today"
                      class="absolute right-1 top-1 inline-flex whitespace-nowrap rounded-full bg-primary-100 px-1.5 py-0.5 text-[9px] font-bold leading-none text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-900/40 dark:text-primary-200 dark:ring-primary-800/60 sm:right-2 sm:top-2 sm:px-2 sm:text-[10px]"
                    >
                      今天
                    </span>
                    <div class="flex min-h-[22px] flex-col items-center">
                      <div class="text-sm font-bold leading-none sm:text-base" :class="day.is_today ? 'text-primary-700 dark:text-primary-300' : 'text-slate-950 dark:text-white'">
                        {{ formatCalendarDay(day.date) }}
                      </div>
                    </div>
                    <span
                      class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold sm:h-8 sm:w-8 sm:text-sm"
                      :class="day.checked_in
                        ? 'bg-gradient-to-br from-primary-400 to-primary-600 text-white shadow-[0_8px_18px_rgba(22,119,255,0.3)]'
                        : day.is_today
                          ? 'bg-white text-primary-600 ring-1 ring-inset ring-primary-200 dark:bg-dark-950 dark:text-primary-300 dark:ring-primary-800'
                          : 'bg-slate-50 text-slate-300 ring-1 ring-inset ring-slate-200 dark:bg-dark-800 dark:text-dark-500 dark:ring-dark-700'"
                    >
                      {{ day.checked_in ? '✓' : day.is_today ? '→' : '' }}
                    </span>
                    <span class="whitespace-nowrap text-[10px] font-semibold text-slate-500 dark:text-dark-400 sm:text-[11px]">
                      {{ formatCalendarWeekday(day.date) }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <transition name="fade">
            <div v-if="message" class="checkin-panel border-primary-200 bg-primary-50/90 p-5 dark:border-primary-900/40 dark:bg-primary-900/20">
              <div class="flex items-start gap-3">
                <div class="rounded-xl bg-primary-100 p-2 text-primary-600 dark:bg-primary-900/40 dark:text-primary-300">
                  <Icon name="checkCircle" size="md" />
                </div>
                <div>
                  <p class="font-semibold text-primary-800 dark:text-primary-200">{{ message }}</p>
                  <p class="mt-1 text-sm text-primary-700 dark:text-primary-300">
                    +{{ formatDollar(lastClaim?.total_reward ?? 0) }}
                    <span v-if="lastClaim"> · {{ lastClaim.current_streak }}天</span>
                  </p>
                </div>
              </div>
            </div>
          </transition>

          <div class="checkin-panel p-5 sm:p-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div class="flex items-start gap-3">
                <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-950/60 dark:text-primary-300 dark:ring-primary-900/40">
                  <Icon name="clipboard" size="md" />
                </div>
                <div>
                  <h3 class="text-lg font-bold tracking-normal text-slate-950 dark:text-white">签到明细</h3>
                  <p class="mt-1 text-sm text-slate-500 dark:text-dark-400">默认展示最近 7 天记录</p>
                </div>
              </div>
              <button class="inline-flex items-center justify-center gap-2 rounded-xl bg-white/85 px-4 py-2 text-sm font-semibold text-slate-700 shadow-sm ring-1 ring-slate-200 transition hover:bg-white dark:bg-dark-900/70 dark:text-dark-200 dark:ring-dark-700" @click="toggleHistory">
                <Icon name="menu" size="sm" />
                {{ showAllHistory ? '收起' : '查看全部' }}
              </button>
            </div>

            <div v-if="history.length === 0" class="mt-5 rounded-2xl border border-dashed border-slate-200 bg-white/50 px-6 py-10 text-center dark:border-dark-700 dark:bg-dark-900/30">
              <div class="mx-auto flex h-24 w-24 items-center justify-center rounded-full bg-gradient-to-br from-primary-50 to-primary-100 text-primary-600 shadow-sm ring-1 ring-primary-100 dark:from-primary-950/50 dark:to-dark-900/50 dark:text-primary-300 dark:ring-primary-900/40">
                <Icon name="search" size="xl" :stroke-width="1.6" />
              </div>
              <p class="mt-4 font-semibold text-slate-800 dark:text-dark-100">最近 7 天暂无签到记录</p>
              <p class="mt-1 text-sm text-slate-500 dark:text-dark-400">坚持签到，记录将在这里展示哦～</p>
            </div>

            <div v-else class="mt-5 overflow-x-auto">
              <table class="w-full min-w-[760px] text-left text-sm">
                <thead>
                  <tr class="border-b border-slate-200 text-slate-500 dark:border-dark-700 dark:text-dark-400">
                    <th class="px-3 py-3 font-semibold">日期</th>
                    <th class="px-3 py-3 font-semibold">连续天数</th>
                    <th class="px-3 py-3 font-semibold">基础奖励</th>
                    <th class="px-3 py-3 font-semibold">额外奖励</th>
                    <th class="px-3 py-3 font-semibold">合计奖励</th>
                    <th class="px-3 py-3 font-semibold">状态</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in history"
                    :key="item.id"
                    class="border-b border-slate-100 last:border-b-0 dark:border-dark-800"
                  >
                    <td class="px-3 py-4 font-medium text-slate-950 dark:text-white">
                      {{ formatDateText(item.checkin_date) }}
                    </td>
                    <td class="px-3 py-4 text-slate-700 dark:text-dark-300">
                      {{ item.streak_days }}
                    </td>
                    <td class="px-3 py-4 text-slate-700 dark:text-dark-300">
                      {{ formatDollar(item.base_reward) }}
                    </td>
                    <td class="px-3 py-4 text-slate-700 dark:text-dark-300">
                      {{ formatDollar(item.extra_reward) }}
                    </td>
                    <td class="px-3 py-4 font-semibold text-primary-600 dark:text-primary-400">
                      {{ formatDollar(item.total_reward) }}
                    </td>
                    <td class="px-3 py-4">
                      <span class="inline-flex items-center rounded-full bg-primary-100 px-2.5 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
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
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import checkinCalendarIcon from '@/assets/checkin-calendar.png'
import { checkinAPI, type CheckinHistoryItem, type CheckinRewardRule, type CheckinStatusResponse } from '@/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateOnly } from '@/utils/format'
import { QQ_GROUP_INVITE_URL, QQ_GROUP_NUMBER } from '@/constants/community'
import { useCheckinReminder } from '@/composables/useCheckinReminder'

const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()
const { markCheckinCompleted } = useCheckinReminder()

const loading = ref(true)
const checkingIn = ref(false)
const showQQBindDialog = ref(false)
const showAllHistory = ref(false)
const status = ref<CheckinStatusResponse | null>(null)
const history = ref<CheckinHistoryItem[]>([])
const lastClaim = ref<{ total_reward: number; current_streak: number; message: string } | null>(null)
const message = ref('')
const checkedDateOverrides = ref(new Set<string>())
const qqGroupNumber = QQ_GROUP_NUMBER
const qqGroupInviteUrl = QQ_GROUP_INVITE_URL

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

const statCards = computed(() => [
  {
    label: '今日可领',
    value: formatRewardRange(status.value?.today_reward_min ?? status.value?.today_reward ?? 0, status.value?.today_reward_max ?? status.value?.today_reward ?? 0),
    hint: `基础奖励 ${formatRewardRange(status.value?.base_reward_min ?? status.value?.base_reward ?? 0, status.value?.base_reward_max ?? status.value?.base_reward ?? 0)}`,
    valueClass: 'text-primary-600 dark:text-primary-300',
  },
  {
    label: '连续签到',
    value: String(status.value?.current_streak ?? 0),
    unit: '天',
    hint: `今天：${alreadyCheckedIn.value ? '已签到' : '未签到'}`,
    valueClass: '',
  },
  {
    label: '本月签到',
    value: String(status.value?.month_checkins ?? 0),
    unit: '天',
    hint: status.value?.timezone || 'Asia/Shanghai',
    valueClass: '',
  },
  {
    label: '累计奖励',
    value: formatDollar(status.value?.total_reward ?? 0),
    hint: `合计可领 ${formatRewardRange(status.value?.today_reward_min ?? status.value?.today_reward ?? 0, status.value?.today_reward_max ?? status.value?.today_reward ?? 0)}`,
    valueClass: 'text-primary-600 dark:text-primary-300',
  },
])

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
    days.set(normalizeDateKey(item.date), {
      checked_in: item.checked_in,
      reward: item.reward
    })
  }
  for (const item of history.value) {
    days.set(normalizeDateKey(item.checkin_date), {
      checked_in: true,
      reward: item.total_reward
    })
  }
  for (const date of checkedDateOverrides.value) {
    const key = normalizeDateKey(date)
    if (!key) continue
    const existing = days.get(key)
    days.set(key, {
      checked_in: true,
      reward: existing?.reward
    })
  }

  const baseDate = parseLocalDate(status.value?.today_date) ?? new Date()
  const out: CalendarDay[] = []
  for (let i = 3; i >= -3; i -= 1) {
    const d = new Date(baseDate)
    d.setDate(baseDate.getDate() - i)
    const date = toLocalDateKey(d)
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

function normalizeDateKey(date?: string | Date | null): string {
  if (!date) return ''
  if (date instanceof Date) return toLocalDateKey(date)
  const match = String(date).match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (match) return `${match[1]}-${match[2]}-${match[3]}`
  const parsed = new Date(date)
  return Number.isNaN(parsed.getTime()) ? String(date) : toLocalDateKey(parsed)
}

function parseLocalDate(date?: string | null): Date | null {
  const key = normalizeDateKey(date)
  if (!key) return null
  const [year, month, day] = key.split('-').map(Number)
  if (!year || !month || !day) return null
  return new Date(year, month - 1, day)
}

function toLocalDateKey(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function mergeCheckinRecord(record?: CheckinHistoryItem): void {
  if (!record?.checkin_date) return
  const dateKey = normalizeDateKey(record.checkin_date)
  if (!dateKey) return
  checkedDateOverrides.value = new Set([...checkedDateOverrides.value, dateKey])

  if (history.value.some((item) => normalizeDateKey(item.checkin_date) === dateKey)) return
  history.value = [
    {
      ...record,
      checkin_date: dateKey,
    },
    ...history.value,
  ]
}

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
  return `$${formatAmountCompact(normalizedMin)}-${formatAmountCompact(normalizedMax)}`
}

function formatAmountCompact(value: number): string {
  const n = Number(value || 0)
  return Number.isInteger(n) ? String(n) : n.toFixed(2)
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

function openQQGroupInvite(): void {
  window.open(qqGroupInviteUrl, '_blank', 'noopener,noreferrer')
}

async function copyQQGroupNumber(): Promise<void> {
  await copyToClipboard(qqGroupNumber, 'QQ群号已复制')
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
    markCheckinCompleted()
    lastClaim.value = {
      total_reward: resp.total_reward,
      current_streak: resp.current_streak,
      message: resp.message,
    }
    if (resp.record) {
      mergeCheckinRecord(resp.record)
    } else if (resp.today_date) {
      checkedDateOverrides.value = new Set([...checkedDateOverrides.value, normalizeDateKey(resp.today_date)])
    }
    message.value = resp.message
    appStore.showSuccess(resp.message)
    await Promise.all([
      authStore.refreshUser().catch(() => undefined),
      loadStatus(),
    ])
    history.value = status.value?.recent_history ?? history.value
    mergeCheckinRecord(resp.record)
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

<style scoped>
.checkin-panel {
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 1.25rem;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 16px 42px rgba(15, 23, 42, 0.08);
  backdrop-filter: blur(18px);
}

.reward-tile {
  box-shadow: none;
  backdrop-filter: none;
}

.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 0.25rem;
}

@media (min-width: 640px) {
  .calendar-grid {
    gap: 0.5rem;
  }
}

</style>

<style>
.dark .checkin-panel {
  border-color: rgba(51, 65, 85, 0.8);
  background: rgba(15, 23, 42, 0.78);
  box-shadow: 0 18px 46px rgba(0, 0, 0, 0.32);
}
</style>
