<template>
  <AppLayout>
    <ScrollablePageLayout content-class="-mx-4 -my-6 px-4 py-8 sm:-mx-6 lg:-mx-8 lg:px-8">
      <div class="lottery-page mx-auto max-w-6xl pb-10">
        <section v-if="currentBroadcast || currentJackpot" class="lottery-broadcast" aria-live="polite">
          <article class="lottery-broadcast-panel lottery-broadcast--recent">
            <div class="lottery-broadcast-heading">
              <div><span class="lottery-broadcast-label">最近抽奖</span><span class="lottery-broadcast-subtitle">最新中奖动态</span></div>
            </div>
            <div v-if="currentBroadcast" class="lottery-broadcast-content" :class="broadcastToneClass(currentBroadcast)">
                <div class="lottery-broadcast-message">
                  <span :key="`recent-user-${currentBroadcast.id}`" class="lottery-broadcast-user lottery-broadcast-dynamic">{{ currentBroadcast.display_name }}</span>
                  <span class="lottery-broadcast-copy">抽中</span>
                  <strong :key="`recent-prize-${currentBroadcast.id}`" class="lottery-broadcast-prize lottery-broadcast-dynamic" :class="broadcastToneClass(currentBroadcast)">{{ currentBroadcast.prize_label }}</strong>
                  <span v-if="isBroadcastJackpot(currentBroadcast)" :key="`recent-jackpot-${currentBroadcast.id}`" class="lottery-broadcast-jackpot lottery-broadcast-dynamic"><Icon name="gift" size="xs" />大奖</span>
                </div>
                <div class="lottery-broadcast-meta">
                  <div class="lottery-broadcast-meta-main">
                    <span class="lottery-broadcast-meta-item"><Icon name="trendingUp" size="xs" /><span class="lottery-broadcast-value-label">价值</span> <strong :key="`recent-value-${currentBroadcast.id}`" class="lottery-broadcast-value lottery-broadcast-dynamic" :class="broadcastToneClass(currentBroadcast)">{{ formatBroadcastValue(currentBroadcast) }}</strong></span>
                    <span class="lottery-broadcast-meta-item is-probability"><Icon name="chart" size="xs" /><span class="lottery-broadcast-value-label">概率</span> <strong :key="`recent-probability-${currentBroadcast.id}`" class="lottery-broadcast-value lottery-broadcast-dynamic" :class="broadcastToneClass(currentBroadcast)">{{ formatBroadcastProbability(currentBroadcast) }}</strong></span>
                  </div>
                  <time class="lottery-broadcast-time"><Icon name="clock" size="xs" /><span>{{ formatBroadcastTime(currentBroadcast.created_at) }}</span></time>
                </div>
            </div>
            <div v-else class="lottery-broadcast-empty">暂无中奖播报</div>
          </article>

          <span class="lottery-broadcast-divider" aria-hidden="true"></span>
          <article class="lottery-broadcast-panel lottery-broadcast--jackpot">
            <div class="lottery-broadcast-heading">
              <div><span class="lottery-broadcast-label">最近大奖</span><span class="lottery-broadcast-subtitle">概率 ≤ 5%</span></div>
            </div>
            <div v-if="currentJackpot" class="lottery-broadcast-content is-jackpot">
              <div class="lottery-broadcast-message">
                <span class="lottery-broadcast-user">{{ currentJackpot.display_name }}</span>
                <span class="lottery-broadcast-copy">抽中</span>
                <strong class="lottery-broadcast-prize is-jackpot">{{ currentJackpot.prize_label }}</strong>
              </div>
              <div class="lottery-broadcast-meta">
                <div class="lottery-broadcast-meta-main">
                  <span class="lottery-broadcast-meta-item"><Icon name="trendingUp" size="xs" /><span class="lottery-broadcast-value-label">价值</span> <strong class="lottery-broadcast-value is-jackpot">{{ formatBroadcastValue(currentJackpot) }}</strong></span>
                  <span class="lottery-broadcast-meta-item is-probability"><Icon name="chart" size="xs" /><span class="lottery-broadcast-value-label">概率</span> <strong class="lottery-broadcast-value is-jackpot">{{ formatBroadcastProbability(currentJackpot) }}</strong></span>
                </div>
                <time class="lottery-broadcast-time"><Icon name="clock" size="xs" /><span>{{ formatBroadcastTime(currentJackpot.created_at) }}</span></time>
              </div>
            </div>
            <div v-else class="lottery-broadcast-empty">暂无概率 ≤ 5% 的大奖</div>
          </article>
        </section>
        <section class="lottery-hero overflow-hidden">
          <div class="lottery-hero-copy">
            <p class="lottery-hero-kicker">幸运福利活动</p>
            <h2>把幸运留给今天</h2>
            <p class="lottery-hero-description">最高可得 $1000 与订阅兑换券</p>

            <div class="lottery-stat-grid">
              <div class="lottery-hero-stat"><span>剩余抽奖次数</span><strong>{{ freeTickets }}<small>次</small></strong></div>
              <div class="lottery-hero-stat"><span>保底进度</span><strong>{{ misses }}<small>/ 5 抽</small></strong></div>
            </div>

            <div class="lottery-hero-rule-list">
              <p><i></i>连续 4 次未中奖，第 5 次必中奖</p>
              <p><i></i>充值抽、邀请抽与付费抽共用保底进度</p>
            </div>
          </div>

          <div class="lottery-wheel-wrap" aria-label="幸运转盘">
            <span class="wheel-pointer" aria-hidden="true"></span>
            <span class="wheel-spark wheel-spark--one" aria-hidden="true"></span>
            <span class="wheel-spark wheel-spark--two" aria-hidden="true"></span>
            <span class="wheel-spark wheel-spark--three" aria-hidden="true"></span>
            <div class="lottery-wheel" :class="{ 'is-drawing': drawing }" :style="wheelStyle">
              <span v-if="wheelJackpotGlowStyle" class="wheel-jackpot-glow" :style="wheelJackpotGlowStyle" aria-hidden="true"></span>
              <span v-for="(prize, index) in wheelPrizes" :key="prize.id" class="wheel-label-position" :style="wheelLabelPositionStyle(index)">
                <span class="wheel-label" :class="prizeToneClass(prize)" :style="wheelLabelTextStyle(index)">{{ prize.label }}</span>
              </span>
            </div>
            <button class="lottery-wheel-action" :class="{ 'is-drawing': drawing }" type="button" :disabled="drawing || purchasing" @click="draw">
              <span class="lottery-wheel-action-label">{{ drawing ? '抽奖中' : '立即抽奖' }}</span>
              <small>{{ drawing ? '幸运加载中' : `剩余 ${freeTickets} 次` }}</small>
            </button>
          </div>
        </section>

        <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div class="lottery-panel p-5 sm:p-6">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
              <div><h2 class="lottery-panel-title">获取更多抽奖次数</h2></div>
            </div>

            <div class="mt-5 grid gap-3 sm:grid-cols-3">
              <article class="lottery-action-card">
                <div class="lottery-action-heading"><div class="lottery-action-icon lottery-action-icon--blue"><Icon name="creditCard" size="md" /></div><h3>每日充值</h3></div>
                <p>今日已获 <span class="font-semibold" :class="dailyTicketProgressClass(rechargeTicketsToday, 2)">{{ rechargeTicketsToday }} / 2</span> 次<br>人民币余额充值或订阅套餐累计 ¥20 +1 次，¥100 再 +1 次</p>
                <button type="button" @click="router.push('/wallet')">去充值 <Icon name="chevronRight" size="xs" /></button>
              </article>
              <article class="lottery-action-card">
                <div ref="inviteRequirementElement" class="lottery-action-heading lottery-invite-heading" @keydown.esc="showInviteRequirement = false">
                  <div class="lottery-action-icon lottery-action-icon--green"><Icon name="gift" size="md" /></div>
                  <h3>邀请好友</h3>
                  <button
                    class="lottery-invite-help-button"
                    type="button"
                    aria-label="查看有效邀请条件"
                    :aria-expanded="showInviteRequirement"
                    aria-controls="lottery-invite-requirement"
                    @click.stop="showInviteRequirement = !showInviteRequirement"
                  >
                    <span aria-hidden="true">?</span>
                  </button>
                  <div v-show="showInviteRequirement" id="lottery-invite-requirement" class="lottery-invite-requirement" role="tooltip">
                    有效邀请需绑定唯一 QQ，累计人民币余额充值或订阅套餐订单满 ¥{{ formatInvitationAmount(invitationFirstPaymentAmount) }}，且实际消费满 ${{ formatInvitationAmount(invitationConsumptionAmount) }}；达成后发放 2 次抽奖机会。
                  </div>
                </div>
                <p>今日已获 <span class="font-semibold text-emerald-600 dark:text-emerald-300">{{ invitationTicketsToday }}</span> 次 · 不限<br>每位有效邀请 +2 次</p>
                <button type="button" @click="router.push('/affiliate')">立即邀请 <Icon name="chevronRight" size="xs" /></button>
              </article>
              <article class="lottery-action-card">
                <div class="lottery-action-heading"><div class="lottery-action-icon lottery-action-icon--amber"><Icon name="sparkles" size="md" /></div><h3>购买抽奖</h3></div>
                <p>今日已获 <span class="font-semibold" :class="dailyTicketProgressClass(purchasedTicketsToday, 5)">{{ purchasedTicketsToday }} / 5</span> 次<br>{{ formattedPurchasePrice }} / 次</p>
                <button type="button" :disabled="!canPurchaseTicket || drawing || purchasing" @click="openPurchaseDialog">{{ remainingPurchases === 0 ? '今日已达上限' : `${formattedPurchasePrice} 购买次数` }} <Icon v-if="remainingPurchases > 0" name="chevronRight" size="xs" /></button>
              </article>
            </div>
          </div>

          <aside class="lottery-panel lottery-ready-panel p-5 sm:p-6">
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-lg font-bold text-slate-950 dark:text-white">获取次数</h2>
            </div>
            <p class="mt-3 text-sm leading-6 text-slate-500 dark:text-dark-300">每次扣除 {{ formattedPurchasePrice }} 账户余额，购买后立即增加 1 次抽奖机会。</p>
            <p v-if="ticketDebt > 0" class="mt-2 text-xs leading-5 text-amber-600 dark:text-amber-300">存在 {{ ticketDebt }} 次待抵扣次数，获得新次数后会优先抵扣。</p>
            <button class="lottery-primary-button mt-5" type="button" @click="router.push('/wallet')"><Icon name="creditCard" size="sm" /><span>充值获取次数</span></button>
            <button class="lottery-secondary-button mt-3" type="button" :disabled="!canPurchaseTicket || drawing || purchasing" @click="openPurchaseDialog"><Icon name="sparkles" size="sm" /><span>{{ formattedPurchasePrice }} 购买次数</span><small class="lottery-purchase-remaining">剩余 {{ remainingPurchases }} 次</small></button>
          </aside>
        </section>

        <section class="mt-5 grid gap-5 lg:grid-cols-[290px_minmax(0,1fr)]">
          <section class="lottery-panel p-5 sm:p-6">
            <div><h2 class="lottery-panel-title">概率明细</h2><p class="lottery-panel-description">各奖项的中奖概率</p></div>
            <div class="mt-5">
              <div class="lottery-probability-list">
                <div v-for="prize in probabilityDetailPrizes" :key="prize.id" class="lottery-probability-item" :class="prizeToneClass(prize)">
                  <span>{{ prize.label }}</span><strong>{{ prize.probability }}%</strong>
                </div>
              </div>
              <div class="lottery-probability-notes">
                <p>活动规则</p>
                <ul>
                  <li>订阅兑换码可在 30 天内兑换；兑换后按奖项标注的时长生效。</li>
                  <li>有效邀请须完成注册、绑定唯一 QQ，累计人民币余额充值或订阅套餐订单满 ¥{{ formatInvitationAmount(invitationFirstPaymentAmount) }}，且实际消费满 ${{ formatInvitationAmount(invitationConsumptionAmount) }}。</li>
                  <li>异常账号将进入风控审核。</li>
                </ul>
              </div>
            </div>
          </section>

          <section class="lottery-panel lottery-recent-panel p-5 sm:p-6">
            <div class="flex items-start justify-between gap-3">
              <div><h2 class="lottery-panel-title">最近抽奖记录</h2><p class="lottery-panel-description">最近 5 次抽奖结果</p></div>
              <button class="lottery-history-button" type="button" @click="showHistoryDialog = true"><Icon name="clock" size="sm" /><span>全部记录</span></button>
            </div>
            <div v-if="history.length" class="lottery-recent-list mt-5">
              <div v-for="item in history.slice(0, 5)" :key="item.id" class="lottery-recent-item">
                <div class="lottery-history-icon" :class="prizeToneClass(item.prize)"><Icon :name="item.prize.kind === 'none' ? 'refresh' : 'gift'" size="sm" /></div>
                <div class="min-w-0 flex-1"><p class="font-semibold" :class="prizeToneClass(item.prize)">{{ item.prize.label }}</p><p v-if="item.isGuaranteed || item.redeemCode" class="mt-0.5 text-xs text-slate-500 dark:text-dark-400"><template v-if="item.isGuaranteed">触发保底</template><template v-if="item.isGuaranteed && item.redeemCode"> · </template><template v-if="item.redeemCode"><span :class="redeemStatusClass(item.redeemStatus)">{{ redeemStatusLabel(item.redeemStatus) }}</span><span v-if="redeemExpiryRemainingLabel(item.redeemStatus, item.redeemExpiresAt)" class="text-slate-400 dark:text-dark-400"> · 兑换截止 {{ redeemExpiryRemainingLabel(item.redeemStatus, item.redeemExpiresAt) }}</span><span v-if="subscriptionValidityLabel(item.subscriptionValidityDays)" class="text-slate-400 dark:text-dark-400"> · {{ subscriptionValidityLabel(item.subscriptionValidityDays) }}</span></template></p><p v-if="item.redeemCode" class="mt-1 truncate font-mono text-[11px] text-violet-600 dark:text-violet-300">兑换码：{{ item.redeemCode }}</p></div>
                <time class="shrink-0 text-right text-[11px] leading-4 tabular-nums text-slate-400 dark:text-dark-400"><span class="block">{{ item.date }}</span><span>{{ item.time }}</span></time>
              </div>
            </div>
            <div v-else class="lottery-recent-empty"><Icon name="clock" size="md" /><span>暂无抽奖记录</span></div>
            <div v-if="history.length" class="lottery-recent-footer">仅展示最近 5 条记录</div>
          </section>
        </section>
      </div>

      <BaseDialog :show="showHistoryDialog" title="抽奖记录" width="normal" @close="showHistoryDialog = false">
        <div v-if="history.length" class="-mx-6 -my-5 divide-y divide-slate-100 dark:divide-dark-700">
          <div v-for="item in history" :key="item.id" class="flex items-center gap-4 px-6 py-4">
            <div class="lottery-history-icon" :class="prizeToneClass(item.prize)"><Icon :name="item.prize.kind === 'none' ? 'refresh' : 'gift'" size="sm" /></div>
            <div class="min-w-0 flex-1"><p class="font-semibold" :class="prizeToneClass(item.prize)">{{ item.prize.label }}</p><p v-if="item.isGuaranteed" class="mt-0.5 text-xs text-slate-500 dark:text-dark-400">触发保底</p><p v-if="item.redeemCode" class="mt-1 font-mono text-[11px] text-violet-600 dark:text-violet-300">兑换码：{{ item.redeemCode }}<span class="ml-2 font-sans font-semibold" :class="redeemStatusClass(item.redeemStatus)">{{ redeemStatusLabel(item.redeemStatus) }}</span><span v-if="redeemExpiryRemainingLabel(item.redeemStatus, item.redeemExpiresAt)" class="ml-2 font-sans text-slate-400 dark:text-dark-400">兑换截止 {{ redeemExpiryRemainingLabel(item.redeemStatus, item.redeemExpiresAt) }}</span><span v-if="subscriptionValidityLabel(item.subscriptionValidityDays)" class="ml-2 font-sans text-slate-400 dark:text-dark-400">{{ subscriptionValidityLabel(item.subscriptionValidityDays) }}</span></p></div>
            <time class="shrink-0 text-right text-[11px] leading-4 tabular-nums text-slate-400 dark:text-dark-400"><span class="block">{{ item.date }}</span><span>{{ item.time }}</span></time>
          </div>
        </div>
        <div v-else class="py-10 text-center text-sm text-slate-500 dark:text-dark-400">还没有抽奖记录</div>
        <template #footer><button class="btn btn-secondary" type="button" @click="showHistoryDialog = false">关闭</button></template>
      </BaseDialog>

      <BaseDialog :show="showResult" :title="resultTitle" width="narrow" @close="closeResultDialog">
        <div v-if="lastResult" class="text-center"><div class="lottery-result-mark" :class="prizeToneClass(lastResult.prize)"><Icon :name="lastResult.prize.kind === 'none' ? 'refresh' : 'gift'" size="xl" /></div><p class="mt-5 text-2xl font-bold" :class="prizeToneClass(lastResult.prize)">{{ lastResult.prize.label }}</p><p class="mt-2 text-sm leading-6 text-slate-500 dark:text-dark-300">{{ lastResult.prize.detail }}</p><div v-if="lastResult.prize.kind === 'quota' && lastResult.balanceBefore !== undefined && lastResult.balanceAfter !== undefined" class="lottery-result-balance-change"><div><span>抽奖前余额</span><strong>{{ formatLotteryBalance(lastResult.balanceBefore) }}</strong></div><div class="is-reward"><span>中奖金额</span><strong>+{{ formatLotteryBalance(lastResult.prize.amount ?? 0) }}</strong></div><div><span>抽奖后余额</span><strong>{{ formatLotteryBalance(lastResult.balanceAfter) }}</strong></div></div><div v-if="lastResult.redeemCode" class="mt-5 rounded-lg border border-violet-200 bg-violet-50 px-4 py-3 text-left dark:border-violet-500/30 dark:bg-violet-950/20"><p class="text-xs font-semibold text-violet-700 dark:text-violet-200">订阅兑换码 <span class="ml-2" :class="redeemStatusClass(lastResult.redeemStatus)">{{ redeemStatusLabel(lastResult.redeemStatus) }}</span></p><div class="mt-2 flex items-center gap-2"><code class="min-w-0 flex-1 break-all text-sm font-semibold text-violet-800 dark:text-violet-100">{{ lastResult.redeemCode }}</code><button class="btn btn-secondary h-8 w-8 shrink-0 p-0" type="button" title="复制兑换码" @click="copyRedeemCode(lastResult.redeemCode)"><Icon name="copy" size="sm" /></button></div><p class="mt-2 text-[11px] text-violet-600 dark:text-violet-300">兑换截止 {{ formatRedeemExpiry(lastResult.redeemExpiresAt) }}<span v-if="redeemExpiryRemainingLabel(lastResult.redeemStatus, lastResult.redeemExpiresAt)"> · {{ redeemExpiryRemainingLabel(lastResult.redeemStatus, lastResult.redeemExpiresAt) }}</span></p><p v-if="subscriptionValidityLabel(lastResult.subscriptionValidityDays)" class="mt-1 text-[11px] text-violet-600 dark:text-violet-300">{{ subscriptionValidityLabel(lastResult.subscriptionValidityDays) }}</p><p class="mt-2 text-xs text-slate-600 dark:text-dark-200">请前往 <RouterLink :to="{ path: '/wallet', query: { tab: 'redeem' } }" class="font-semibold text-primary-600 hover:text-primary-700 dark:text-primary-300">我的钱包</RouterLink> 进行兑换</p></div><p v-if="lastResult.isGuaranteed" class="mt-3 text-xs font-semibold text-primary-600 dark:text-primary-300">本次已触发 5 抽保底</p></div>
        <template #footer><button class="btn btn-primary" type="button" @click="closeResultDialog">完成</button></template>
      </BaseDialog>

      <BaseDialog :show="showPurchaseDialog" title="确认购买抽奖次数" width="narrow" @close="closePurchaseDialog">
        <div class="lottery-purchase-summary">
          <div><span>支付金额</span><strong>{{ formattedPurchasePrice }}</strong></div>
          <div><span>支付后余额</span><strong>${{ balanceAfterPurchase.toFixed(2) }}</strong></div>
          <div><span>到账内容</span><strong>抽奖次数 +1</strong></div>
        </div>
        <p class="mt-4 text-sm leading-6 text-slate-500 dark:text-dark-300">付款后将从可用额度中扣除 {{ formattedPurchasePrice }}，并立即增加 1 次抽奖机会。</p>
        <template #footer>
          <button class="btn btn-secondary" type="button" :disabled="purchasing" @click="closePurchaseDialog">取消</button>
          <button class="btn btn-primary" type="button" :disabled="purchasing" @click="confirmPurchase">{{ purchasing ? '处理中' : '确认付款' }}</button>
        </template>
      </BaseDialog>
    </ScrollablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ScrollablePageLayout from '@/components/layout/ScrollablePageLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  clearPendingLotteryRequestID,
  getOrCreatePendingLotteryRequestID,
  getWheelRotationForPrize,
  lotteryPrizeFromSnapshot,
  lotteryPrizePool,
  readPendingLotteryRequestID,
  type LotteryPrize,
  type LotteryRequestStorage,
} from '@/utils/lottery'
import { useAppStore, useAuthStore } from '@/stores'
import { useLotteryState } from '@/composables/useLotteryState'
import { lotteryAPI, type LotteryDraw, type LotteryPrizeConfig, type LotteryRecentWinner } from '@/api/lottery'

type DrawHistoryItem = { id: number; prize: LotteryPrize; isGuaranteed: boolean; date: string; time: string; redeemCode?: string; redeemStatus?: LotteryDraw['redeem_status']; redeemExpiresAt?: string; subscriptionValidityDays?: number }
type DisplayResult = { prize: LotteryPrize; isGuaranteed: boolean; balanceBefore?: number; balanceAfter?: number; redeemCode?: string; redeemStatus?: LotteryDraw['redeem_status']; redeemExpiresAt?: string; subscriptionValidityDays?: number }

const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { freeTickets, lotteryEnabled, remainingPurchases, rechargeTicketsToday, invitationTicketsToday, purchasedTicketsToday, misses, ticketDebt, applyLotteryStatus } = useLotteryState()
const drawing = ref(false)
const purchasing = ref(false)
const wheelRotation = ref(0)
const showResult = ref(false)
const showHistoryDialog = ref(false)
const showPurchaseDialog = ref(false)
const showInviteRequirement = ref(false)
const inviteRequirementElement = ref<HTMLElement | null>(null)
const invitationFirstPaymentAmount = ref(20)
const invitationConsumptionAmount = ref(100)
const purchasePrice = ref(30)
const lastResult = ref<DisplayResult | null>(null)
const history = ref<DrawHistoryItem[]>([])
const recentWinners = ref<LotteryRecentWinner[]>([])
const broadcastIndex = ref(0)
const pendingHistoryItem = ref<DrawHistoryItem | null>(null)
const prizePool = ref<LotteryPrize[]>(lotteryPrizePool)
let prizePoolRefreshTimer: number | undefined
let broadcastTimer: number | undefined
const purchaseRequestStorageKey = 'lottery:purchase:pending-request-id'
const drawRequestStorageKey = 'lottery:draw:pending-request-id'
const requestStorage = getLotteryRequestStorage()
const purchaseRequestID = ref(readPendingLotteryRequestID(requestStorage, purchaseRequestStorageKey))
const drawRequestID = ref(readPendingLotteryRequestID(requestStorage, drawRequestStorageKey))
const wheelPrizes = computed(() => prizePool.value)
const probabilityDetailPrizes = computed(() =>
  prizePool.value
    .map((prize, index) => ({ prize, index }))
    .sort((left, right) => right.prize.probability - left.prize.probability || left.index - right.index)
    .map(({ prize }) => prize),
)
const jackpotPrizeID = computed(() => {
  const rewards = wheelPrizes.value.filter((prize) => prize.kind !== 'none' && Number(prize.probability) > 0)
  return rewards.reduce<LotteryPrize | null>((lowestProbabilityPrize, prize) => {
    if (!lowestProbabilityPrize || prize.probability < lowestProbabilityPrize.probability) return prize
    return lowestProbabilityPrize
  }, null)?.id
})

function wheelSegmentColor(prize: LotteryPrize, index: number): string {
  if (prize.id === jackpotPrizeID.value) return '#f3b327'
  if (prize.kind === 'none') return '#d9e3ee'
  if (prize.kind === 'voucher') return index % 2 ? '#dec7f2' : '#ead9f8'
  return ['#cce6fb', '#a9d4f6', '#c2e1f8', '#b5dbf6'][index % 4]
}

const wheelStyle = computed(() => {
  const count = Math.max(wheelPrizes.value.length, 1)
  const segment = 360 / count
  const stops = wheelPrizes.value.map((prize, index) => {
    const start = index * segment
    const end = (index + 1) * segment
    if (prize.id === jackpotPrizeID.value) {
      const middle = start + segment / 2
      return `#fff7cf ${start}deg, #ffd96a ${middle}deg, #e7a214 ${end}deg`
    }
    return `${wheelSegmentColor(prize, index)} ${start}deg ${end}deg`
  }).join(',')
  const fallbackStops = '#d9e3ee 0deg 360deg'
  return { transform: `rotate(${wheelRotation.value}deg)`, background: `conic-gradient(from -${segment / 2}deg,${stops || fallbackStops})` }
})

const wheelJackpotGlowStyle = computed<Record<string, string> | null>(() => {
  const jackpotIndex = wheelPrizes.value.findIndex((prize) => prize.id === jackpotPrizeID.value)
  if (jackpotIndex < 0) return null

  const segment = 360 / wheelPrizes.value.length
  const start = jackpotIndex * segment
  const end = (jackpotIndex + 1) * segment
  return { background: `conic-gradient(from -${segment / 2}deg, transparent 0deg ${start}deg, rgba(255, 198, 44, .98) ${start}deg ${end}deg, transparent ${end}deg 360deg)` }
})
const resultTitle = computed(() => lastResult.value?.prize.kind === 'none' ? '再接再厉' : '恭喜中奖')
const availableBalance = computed(() => Number(authStore.user?.balance || 0))
const formattedPurchasePrice = computed(() => formatLotteryPurchasePrice(purchasePrice.value))
const balanceAfterPurchase = computed(() => Math.max(availableBalance.value - purchasePrice.value, 0))
const canPurchaseTicket = computed(() => remainingPurchases.value > 0 && availableBalance.value >= purchasePrice.value)
const recentBroadcastItems = computed(() => {
  const winners = recentWinners.value.filter((winner) => winner.prize_type !== 'none')
  if (!winners.length) return []
  return winners.slice(0, 10)
})
const jackpotItems = computed(() => recentWinners.value.filter((winner) => isBroadcastJackpot(winner)).slice(0, 1))
const currentBroadcast = computed(() => recentBroadcastItems.value[broadcastIndex.value % Math.max(recentBroadcastItems.value.length, 1)] ?? null)
const currentJackpot = computed(() => jackpotItems.value[0] ?? null)

function wheelLabelPositionStyle(index: number): Record<string, string> {
  const angle = index * (360 / Math.max(wheelPrizes.value.length, 1))
  return { transform: `rotate(${angle}deg) translateY(-98px)` }
}

function wheelLabelTextStyle(index: number): Record<string, string> {
  const angle = index * (360 / Math.max(wheelPrizes.value.length, 1))
  return { transform: `translate(-50%, -50%) rotate(${-angle}deg)` }
}

function prizeToneClass(prize: LotteryPrize): string {
  if (prize.id === jackpotPrizeID.value) return 'is-jackpot'
  return `is-${prize.kind}`
}

function broadcastToneClass(winner: LotteryRecentWinner): string {
  if (isBroadcastJackpot(winner)) return 'is-jackpot'
  return winner.prize_type === 'subscription' ? 'is-voucher' : 'is-quota'
}

function isBroadcastJackpot(winner: LotteryRecentWinner): boolean {
  const probability = Number(winner.probability)
  return probability > 0 && probability <= 0.05
}

function broadcastWeight(winner: LotteryRecentWinner): number {
  return isBroadcastJackpot(winner) ? 5 : 1
}

function selectRandomBroadcast(excludeCurrent = true): void {
  const items = recentBroadcastItems.value
  if (!items.length) return
  const currentID = excludeCurrent ? currentBroadcast.value?.id : undefined
  const candidates = items.filter((winner) => winner.id !== currentID)
  const selectable = candidates.length ? candidates : items
  const totalWeight = selectable.reduce((total, winner) => total + broadcastWeight(winner), 0)
  let target = Math.random() * totalWeight
  let selected = selectable[selectable.length - 1]
  for (const winner of selectable) {
    target -= broadcastWeight(winner)
    if (target <= 0) {
      selected = winner
      break
    }
  }
  broadcastIndex.value = items.findIndex((winner) => winner.id === selected.id)
}

function formatBroadcastTime(value: string): string {
  const elapsed = Math.max(0, Date.now() - new Date(value).getTime())
  const minutes = Math.floor(elapsed / 60_000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  return `${Math.floor(hours / 24)} 天前`
}

function formatBroadcastValue(winner: LotteryRecentWinner): string {
  const amount = Number(winner.amount)
  return `¥${Number.isFinite(amount) ? amount.toLocaleString('zh-CN', { maximumFractionDigits: 2 }) : '0'}`
}

function formatBroadcastProbability(winner: LotteryRecentWinner): string {
  const probability = Number(winner.probability)
  if (!Number.isFinite(probability)) return '—'
  const percentage = probability * 100
  return `${percentage.toLocaleString('zh-CN', { maximumFractionDigits: 2 })}%`
}

function formatDrawTimestamp(value: string): { date: string; time: string } {
  const now = new Date(value)
  const pad = (value: number): string => String(value).padStart(2, '0')
  return {
    date: `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`,
    time: `${pad(now.getHours())}:${pad(now.getMinutes())}`,
  }
}

function formatRedeemExpiry(value?: string): string {
  return value ? formatDrawTimestamp(value).date : '待确认'
}

function redeemExpiryRemainingLabel(status: LotteryDraw['redeem_status'] | undefined, expiresAt?: string): string {
  if (status !== 'unused' || !expiresAt) return ''
  const remainingMs = new Date(expiresAt).getTime() - Date.now()
  if (remainingMs <= 0) return '已过期'
  return `剩余 ${Math.ceil(remainingMs / 86_400_000)} 天`
}

function subscriptionValidityLabel(days?: number): string {
  return days && days > 0 ? `兑换后使用 ${days} 天` : ''
}

function displayPrize(draw: LotteryDraw): LotteryPrize {
	return lotteryPrizeFromSnapshot(draw)
}

function displayHistory(draw: LotteryDraw): DrawHistoryItem {
  return { id: draw.id, prize: displayPrize(draw), isGuaranteed: draw.guaranteed, redeemCode: draw.redeem_code, redeemStatus: draw.redeem_status, redeemExpiresAt: draw.redeem_expires_at, subscriptionValidityDays: draw.subscription_validity_days, ...formatDrawTimestamp(draw.created_at) }
}

function mapPrizeConfig(config: LotteryPrizeConfig[]): LotteryPrize[] {
  const totalProbability = config.reduce((total, prize) => total + Number(prize.probability || 0), 0)
  if (Math.abs(totalProbability - 1) > 0.000001) return lotteryPrizePool
  return config.map((prize) => ({
    id: prize.id,
    label: prize.label,
    detail: prize.type === 'subscription' ? '订阅兑换券已发放' : prize.type === 'none' ? '下次好运会来' : '奖励已发放到账户',
    probability: Number((Number(prize.probability) * 100).toFixed(3)),
    kind: prize.type === 'subscription' ? 'voucher' : prize.type === 'balance' ? 'quota' : 'none',
    amount: prize.amount,
  }))
}

function formatInvitationAmount(amount: number): string {
  const normalized = Math.max(0, Number(amount) || 0)
  return Number.isInteger(normalized) ? String(normalized) : normalized.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}

function formatLotteryPurchasePrice(amount: number): string {
  return `$${formatInvitationAmount(amount)}`
}

function redeemStatusLabel(status?: LotteryDraw['redeem_status']): string {
  if (status === 'used') return '已兑换'
  if (status === 'expired') return '已过期'
  return '未兑换'
}

function redeemStatusClass(status?: LotteryDraw['redeem_status']): string {
  return status === 'unused' ? 'text-emerald-600 dark:text-emerald-300' : status === 'expired' ? 'text-slate-400 dark:text-dark-400' : 'text-slate-600 dark:text-dark-300'
}

function dailyTicketProgressClass(current: number, limit: number): string {
  return current >= limit ? 'text-red-600 dark:text-red-300' : 'text-emerald-600 dark:text-emerald-300'
}

function formatLotteryBalance(amount: number): string {
  return `$${Number(amount).toFixed(2)}`
}

async function copyRedeemCode(code: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(code)
    appStore.showSuccess('兑换码已复制')
  } catch {
    appStore.showError('复制兑换码失败')
  }
}

function getLotteryRequestStorage(): LotteryRequestStorage | undefined {
  try {
    return window.sessionStorage
  } catch {
    return undefined
  }
}

function ensurePurchaseRequestID(): string {
  if (!purchaseRequestID.value) {
    purchaseRequestID.value = getOrCreatePendingLotteryRequestID(requestStorage, purchaseRequestStorageKey)
  }
  return purchaseRequestID.value
}

function clearPurchaseRequestID(expectedRequestID?: string): void {
  clearPendingLotteryRequestID(requestStorage, purchaseRequestStorageKey, expectedRequestID)
  if (!expectedRequestID || purchaseRequestID.value === expectedRequestID) purchaseRequestID.value = undefined
}

function ensureDrawRequestID(): string {
  if (!drawRequestID.value) {
    drawRequestID.value = getOrCreatePendingLotteryRequestID(requestStorage, drawRequestStorageKey)
  }
  return drawRequestID.value
}

function clearDrawRequestID(expectedRequestID?: string): void {
  clearPendingLotteryRequestID(requestStorage, drawRequestStorageKey, expectedRequestID)
  if (!expectedRequestID || drawRequestID.value === expectedRequestID) drawRequestID.value = undefined
}

async function refreshLottery(): Promise<void> {
  const recentWinnersRequest = lotteryAPI.listRecentWinners
    ? lotteryAPI.listRecentWinners(30).catch(() => ({ data: [] as LotteryRecentWinner[] }))
    : Promise.resolve({ data: [] as LotteryRecentWinner[] })
  const [statusResponse, drawsResponse, prizeResponse, recentWinnersResponse] = await Promise.all([lotteryAPI.getStatus(), lotteryAPI.listDraws(50), lotteryAPI.getPrizePool(), recentWinnersRequest])
  applyLotteryStatus(statusResponse.data)
  if (!lotteryEnabled.value) {
    appStore.showWarning('幸运抽奖暂未开启')
    await router.replace('/dashboard')
    return
  }
  if (drawRequestID.value && drawsResponse.data.some((draw) => draw.request_id === drawRequestID.value)) {
    clearDrawRequestID(drawRequestID.value)
  }
  history.value = drawsResponse.data.map(displayHistory)
  setRecentWinners(recentWinnersResponse.data)
  prizePool.value = mapPrizeConfig(prizeResponse.data.prizes)
  invitationFirstPaymentAmount.value = Number(prizeResponse.data.invitation_first_payment_amount) || 20
  invitationConsumptionAmount.value = Number(prizeResponse.data.invitation_consumption_amount) || 100
  purchasePrice.value = Number(prizeResponse.data.purchase_price) || 30
}

async function refreshPrizePool(): Promise<void> {
  const response = await lotteryAPI.getPrizePool()
  prizePool.value = mapPrizeConfig(response.data.prizes)
  invitationFirstPaymentAmount.value = Number(response.data.invitation_first_payment_amount) || 20
  invitationConsumptionAmount.value = Number(response.data.invitation_consumption_amount) || 100
  purchasePrice.value = Number(response.data.purchase_price) || 30
}

async function refreshLotteryStatus(): Promise<void> {
  const response = await lotteryAPI.getStatus()
  applyLotteryStatus(response.data)
}

async function refreshRecentWinners(): Promise<void> {
  if (!lotteryAPI.listRecentWinners) return
  const response = await lotteryAPI.listRecentWinners(30)
  setRecentWinners(response.data)
}

function setRecentWinners(items: LotteryRecentWinner[]): void {
  const currentID = currentBroadcast.value?.id
  recentWinners.value = items
  const preservedIndex = currentID === undefined ? -1 : recentBroadcastItems.value.findIndex((winner) => winner.id === currentID)
  if (preservedIndex >= 0) {
    broadcastIndex.value = preservedIndex
  } else {
    broadcastIndex.value = 0
    selectRandomBroadcast(false)
  }
}

function closeResultDialog(): void {
  showResult.value = false
  if (!pendingHistoryItem.value) return

  const item = pendingHistoryItem.value
  history.value = [item, ...history.value.filter((historyItem) => historyItem.id !== item.id)].slice(0, 50)
  pendingHistoryItem.value = null
}

function openPurchaseDialog(): void {
	if (purchasing.value) return
  if (remainingPurchases.value <= 0) {
    appStore.showError('今日购买抽奖次数已达上限')
    return
  }
  if (availableBalance.value < purchasePrice.value) {
    appStore.showError('账户余额不足，无法购买抽奖次数')
    return
  }
  showPurchaseDialog.value = true
}

function closePurchaseDialog(): void {
  if (purchasing.value) return
  showPurchaseDialog.value = false
  clearPurchaseRequestID()
}

async function confirmPurchase(): Promise<void> {
	if (purchasing.value) return
  if (!canPurchaseTicket.value) {
    showPurchaseDialog.value = false
    openPurchaseDialog()
    return
  }
  const requestID = ensurePurchaseRequestID()
  purchasing.value = true
  try {
    const response = await lotteryAPI.purchaseTicket(requestID)
    clearPurchaseRequestID(requestID)
    applyLotteryStatus(response.data)
    showPurchaseDialog.value = false
    appStore.showSuccess('购买成功，已增加 1 次抽奖机会')
    await authStore.refreshUser().catch(() => undefined)
  } catch (error: any) {
    appStore.showError(error?.message || '购买抽奖次数失败')
  } finally {
    purchasing.value = false
  }
}

async function draw(): Promise<void> {
  if (drawing.value) return
  if (freeTickets.value <= 0) {
    appStore.showInfo(`暂无剩余抽奖次数，可使用 ${formattedPurchasePrice.value} 购买 1 次`)
    return
  }
  drawing.value = true
  try {
    await refreshPrizePool()
    const requestID = ensureDrawRequestID()
    const response = await lotteryAPI.draw(requestID)
    clearDrawRequestID(requestID)
    const result = response.data
    const prize = displayPrize(result)
    const targetIndex = wheelPrizes.value.findIndex((item) => item.id === result.prize_id)
    wheelRotation.value = getWheelRotationForPrize(wheelRotation.value, Math.max(0, targetIndex), wheelPrizes.value.length)
    lastResult.value = {
      prize,
      isGuaranteed: result.guaranteed,
      balanceBefore: result.balance_before,
      balanceAfter: result.balance_after,
      redeemCode: result.redeem_code,
      redeemStatus: result.redeem_status,
      redeemExpiresAt: result.redeem_expires_at,
      subscriptionValidityDays: result.subscription_validity_days,
    }
    pendingHistoryItem.value = displayHistory(result)
    await new Promise<void>((resolve) => window.setTimeout(resolve, 1600))
    const [statusResult] = await Promise.allSettled([lotteryAPI.getStatus(), authStore.refreshUser()])
    if (statusResult.status === 'fulfilled') applyLotteryStatus(statusResult.value.data)
    drawing.value = false
    showResult.value = true
  } catch (error: any) {
    drawing.value = false
    appStore.showError(error?.message || '抽奖失败，请稍后重试')
  }
}

onMounted(() => {
  document.addEventListener('click', closeInviteRequirementOnOutsideClick)
  refreshLottery().catch((error: any) => appStore.showError(error?.message || '加载抽奖状态失败'))
  prizePoolRefreshTimer = window.setInterval(() => {
    if (document.visibilityState !== 'visible') return
    refreshPrizePool().catch(() => undefined)
    // Invitation rewards are reconciled by the status endpoint. Poll it while
    // the page is open so newly qualified invitees do not require a reload.
    refreshLotteryStatus().catch(() => undefined)
    refreshRecentWinners().catch(() => undefined)
  }, 30000)
  broadcastTimer = window.setInterval(() => {
    if (document.visibilityState !== 'visible') return
    if (recentBroadcastItems.value.length > 1) selectRandomBroadcast()
  }, 3400)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeInviteRequirementOnOutsideClick)
  if (prizePoolRefreshTimer !== undefined) window.clearInterval(prizePoolRefreshTimer)
  if (broadcastTimer !== undefined) window.clearInterval(broadcastTimer)
})

function closeInviteRequirementOnOutsideClick(event: MouseEvent): void {
  const target = event.target
  if (target instanceof Node && !inviteRequirementElement.value?.contains(target)) {
    showInviteRequirement.value = false
  }
}
</script>

<style scoped>
.lottery-page { --lottery-blue:#1677ff; --lottery-ink:#17294e; --lottery-line:rgba(205, 221, 239, .9); }
.lottery-broadcast { display:grid; min-width:0; min-height:72px; grid-template-columns:minmax(0,1fr) 1px minmax(0,1fr); align-items:center; column-gap:14px; margin-bottom:13px; border:1px solid #dce6f2; border-radius:14px; background:linear-gradient(105deg,#fff 0%,#fbfdff 58%,#f7fbff 100%); padding:10px 14px; box-shadow:0 10px 24px rgba(38,91,158,.08); }
.lottery-broadcast-panel { display:grid; min-width:0; grid-template-columns:max-content minmax(0,1fr); align-items:center; gap:14px; }
.lottery-broadcast--jackpot .lottery-broadcast-label { color:#9b6400; }
.lottery-broadcast-label { display:block; color:#344d70; font-size:13px; font-weight:850; letter-spacing:.01em; white-space:nowrap; }
.lottery-broadcast-subtitle { display:block; margin-top:3px; color:#8494ab; font-size:10px; white-space:nowrap; }
.lottery-broadcast-divider { width:1px; height:48px; align-self:center; background:#e2eaf4; }
.lottery-broadcast-content { display:grid; min-width:0; margin-top:0; grid-template-columns:minmax(0,4fr) minmax(126px,1fr); grid-template-areas:'message meta'; align-items:center; gap:4px 10px; overflow:hidden; color:#71819a; font-size:14px; white-space:nowrap; }
.lottery-broadcast-dynamic { display:inline-block; }
.lottery-broadcast--recent .lottery-broadcast-dynamic { animation:lottery-broadcast-field-in .26s ease both; }
.lottery-broadcast-message { display:flex; min-width:0; align-items:center; justify-content:center; gap:8px; overflow:hidden; }
.lottery-broadcast-content .lottery-broadcast-message { grid-area:message; }
.lottery-broadcast-meta { display:grid; width:100%; min-width:126px; grid-template-columns:minmax(0,1fr) 40px; align-items:center; gap:8px; grid-area:meta; justify-self:end; padding-left:2px; line-height:1.15; text-align:left; }
.lottery-broadcast-meta-main { display:grid; min-width:0; justify-items:start; gap:4px; }
.lottery-broadcast-meta-item { display:inline-flex; min-width:0; align-items:center; justify-content:flex-start; gap:4px; }
.lottery-broadcast-meta-item svg { width:14px; height:14px; flex:0 0 auto; color:#8fa0b6; }
.lottery-broadcast-time { display:flex; min-width:0; flex-direction:column; align-items:center; justify-content:center; gap:3px; color:#7f91aa; font-size:10px; white-space:nowrap; }
.lottery-broadcast-time svg { width:14px; height:14px; color:#8fa0b6; }
.lottery-broadcast-user { min-width:0; max-width:50%; overflow:hidden; color:#243d60; font-size:13px; font-weight:800; text-overflow:ellipsis; white-space:nowrap; }
.lottery-broadcast-copy { color:#74849a; font-size:13px; }
.lottery-broadcast-prize { min-width:0; max-width:48%; overflow:hidden; color:#1260bd; font-size:15px; font-weight:850; text-overflow:ellipsis; white-space:nowrap; }
.lottery-broadcast-content.is-voucher .lottery-broadcast-prize { color:#7c3aa8; }
.lottery-broadcast-content.is-jackpot .lottery-broadcast-prize { color:#c27a00; font-size:15px; font-weight:900; text-shadow:0 0 8px rgba(238,177,30,.25); }
.lottery-broadcast-value-label { font-size:10px; }
.lottery-broadcast-value { font-size:12px; font-variant-numeric:tabular-nums; font-weight:850; }
.lottery-broadcast-meta-item:not(.is-probability),.lottery-broadcast-meta-item:not(.is-probability) .lottery-broadcast-value-label,.lottery-broadcast-meta-item:not(.is-probability) .lottery-broadcast-value,.lottery-broadcast-meta-item:not(.is-probability) svg { color:#d84a56; }
.lottery-broadcast-meta-item.is-probability,.lottery-broadcast-meta-item.is-probability .lottery-broadcast-value-label,.lottery-broadcast-meta-item.is-probability .lottery-broadcast-value,.lottery-broadcast-meta-item.is-probability svg { color:#1260bd; }
.lottery-broadcast-jackpot { display:inline-flex; align-items:center; gap:4px; border:1px solid #f0c34e; border-radius:999px; background:#fff8df; color:#a86a00; padding:3px 9px; font-size:11px; font-variant-numeric:tabular-nums; font-weight:850; }
.lottery-broadcast-jackpot svg { width:13px; height:13px; flex:0 0 auto; }
.lottery-broadcast-probability { color:#a96d00; font-size:10px; font-variant-numeric:tabular-nums; font-weight:800; white-space:nowrap; }
.lottery-broadcast-empty { color:#9aacca; font-size:10px; }
.lottery-broadcast-slide-enter-active,.lottery-broadcast-slide-leave-active { transition:opacity .2s ease, transform .2s ease; }
.lottery-broadcast-slide-enter-from { opacity:0; transform:translateY(6px); }
.lottery-broadcast-slide-leave-to { opacity:0; transform:translateY(-6px); }
.lottery-eyebrow { color:#1677ff; font-size:11px; font-weight:750; letter-spacing:.08em; text-transform:uppercase; }
.lottery-history-button { display:inline-flex; align-items:center; justify-content:center; gap:.5rem; min-height:36px; border:1px solid var(--lottery-line); border-radius:8px; background:#fff; color:#58708f; padding:0 .8rem; font-size:.8125rem; font-weight:650; transition:.2s ease; }
.lottery-history-button:hover { border-color:#a8cdfc; color:#1677ff; background:#f7fbff; }
.lottery-hero { position:relative; display:grid; grid-template-columns:minmax(0,1fr) minmax(320px, .86fr); min-height:410px; border:1px solid rgba(130,184,255,.72); border-radius:16px; background:linear-gradient(120deg,#2f79ed 0%,#4f9df2 53%,#69c8d7 100%); box-shadow:0 18px 42px rgba(45,107,199,.19), inset 0 1px rgba(255,255,255,.36); isolation:isolate; }
.lottery-hero::before { position:absolute; z-index:-1; right:-110px; top:-140px; width:420px; height:420px; border-radius:50%; background:rgba(214,244,255,.19); content:''; filter:blur(10px); }
.lottery-hero-copy { display:flex; flex-direction:column; justify-content:center; padding:36px 20px 34px 36px; }.lottery-hero-kicker { color:#dcefff; font-size:12px; font-weight:750; letter-spacing:.1em; }.lottery-hero h2 { margin-top:10px; color:#fff; font-size:32px; font-weight:750; line-height:1.15; }.lottery-hero-description { margin-top:9px; color:#f1f8ff; font-size:14px; }
.lottery-stat-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:12px; max-width:430px; margin-top:26px; }.lottery-hero-stat { border:1px solid rgba(255,255,255,.25); border-radius:10px; background:rgba(255,255,255,.15); padding:13px 16px; }.lottery-hero-stat span { display:block; color:#e1f0ff; font-size:12px; }.lottery-hero-stat strong { display:block; margin-top:4px; color:#fff; font-size:27px; line-height:1; }.lottery-hero-stat small { margin-left:4px; font-size:12px; font-weight:650; }
.lottery-hero-rule-list { display:grid; gap:10px; max-width:430px; margin-top:24px; padding-top:18px; border-top:1px solid rgba(255,255,255,.24); }.lottery-hero-rule-list p { display:flex; align-items:center; gap:10px; color:#fff; font-size:13px; }.lottery-hero-rule-list i { width:7px; height:7px; border-radius:50%; background:#fff; }.lottery-hero-note { margin-top:28px; color:#ddf0ff; font-size:12px; }
.lottery-wheel-wrap { position:relative; display:grid; min-height:370px; place-items:center; padding:30px; }.wheel-pointer { position:absolute; z-index:5; top:26px; width:0; height:0; border-right:17px solid transparent; border-bottom:31px solid #fff; border-left:17px solid transparent; filter:drop-shadow(0 3px 5px rgba(19,76,145,.22)); animation:lottery-pointer-breathe 2.4s ease-in-out infinite; }.lottery-wheel-wrap::before { position:absolute; z-index:0; width:332px; height:332px; border:1px solid rgba(255,255,255,.22); border-radius:50%; background:radial-gradient(circle,rgba(255,255,255,.19) 0%,rgba(221,246,255,.09) 44%,transparent 68%); box-shadow:0 0 48px rgba(230,250,255,.18); content:''; animation:lottery-halo-breathe 3.2s ease-in-out infinite; }.lottery-wheel { position:relative; z-index:2; width:310px; height:310px; border:8px solid rgba(255,255,255,.94); border-radius:50%; background:conic-gradient(from -25.714deg,#ecf8ff 0deg 51.4deg,#afd9fb 51.4deg 102.8deg,#e9f7ff 102.8deg 154.2deg,#c6e7fe 154.2deg 205.6deg,#edf8ff 205.6deg 257deg,#afd9fb 257deg 308.4deg,#d9efff 308.4deg 360deg); box-shadow:0 0 0 11px rgba(255,255,255,.18),0 13px 26px rgba(19,85,167,.2); transition:transform 1.6s cubic-bezier(.12,.77,.19,1),filter .3s ease,box-shadow .3s ease; }.lottery-wheel.is-drawing { filter:saturate(1.12) brightness(1.04); box-shadow:0 0 0 11px rgba(255,255,255,.24),0 18px 34px rgba(17,74,153,.29); }.lottery-wheel::before { position:absolute; z-index:2; inset:50% auto auto 50%; width:100px; height:100px; border:5px solid #d9efff; border-radius:50%; background:#fff; box-shadow:inset 0 1px rgba(255,255,255,.9); content:''; transform:translate(-50%,-50%); }.lottery-wheel::after { position:absolute; z-index:1; inset:11px; border:1px solid rgba(68,135,205,.22); border-radius:50%; box-shadow:inset 0 0 0 1px rgba(255,255,255,.55); content:''; pointer-events:none; }.wheel-jackpot-glow { position:absolute; z-index:0; inset:-18px; border-radius:50%; filter:blur(13px); mix-blend-mode:screen; opacity:.82; pointer-events:none; animation:lottery-jackpot-glow 2.2s ease-in-out infinite; }.wheel-label-position { position:absolute; z-index:1; left:50%; top:50%; width:0; height:0; }.wheel-label { position:absolute; display:grid; width:92px; min-height:28px; place-items:center; color:#2766af; font-size:12px; font-weight:750; line-height:1.2; text-align:center; text-shadow:0 1px rgba(255,255,255,.58); }.wheel-spark { position:absolute; z-index:1; width:7px; height:7px; border-radius:50%; background:rgba(255,255,255,.94); box-shadow:0 0 14px rgba(255,255,255,.86); animation:lottery-spark-drift 2.9s ease-in-out infinite; }.wheel-spark--one { right:49px; top:79px; }.wheel-spark--two { bottom:70px; left:59px; width:5px; height:5px; animation-delay:-1.2s; }.wheel-spark--three { right:72px; bottom:58px; width:4px; height:4px; animation-delay:-2.1s; }.lottery-wheel-action { position:absolute; z-index:4; display:grid; place-items:center; width:78px; height:78px; border:0; border-radius:50%; background:#1677ff; color:#fff; box-shadow:0 9px 18px rgba(22,119,255,.27); transition:.2s ease; animation:lottery-action-breathe 2.8s ease-in-out infinite; }.lottery-wheel-action:not(:disabled):hover { background:#0958d9; transform:scale(1.04); animation-play-state:paused; }.lottery-wheel-action:disabled { cursor:not-allowed; opacity:.85; animation:none; }.lottery-wheel-action span { font-size:14px; font-weight:750; }.lottery-wheel-action small { margin-top:3px; color:#ddecff; font-size:10px; }
.lottery-panel { border:1px solid var(--lottery-line); border-radius:14px; background:rgba(255,255,255,.9); box-shadow:0 12px 30px rgba(38,91,158,.06); }.lottery-panel-heading { display:flex; align-items:center; gap:9px; }.lottery-panel-heading-icon { display:grid; width:27px; height:27px; place-items:center; border-radius:7px; background:#e8f2ff; color:#1677ff; }.lottery-panel-title { color:var(--lottery-ink); font-size:17px; font-weight:750; }.lottery-panel-description { margin-top:4px; color:#71819a; font-size:13px; }.lottery-limit-badge,.lottery-guarantee-badge { display:inline-flex; align-items:center; align-self:start; border-radius:999px; background:#e8f2ff; color:#1677ff; padding:6px 10px; font-size:11px; font-weight:750; }
.lottery-action-card { min-height:154px; border:1px solid #dce8f5; border-radius:10px; background:#f8fbff; padding:16px; }.lottery-action-heading { display:flex; align-items:center; gap:10px; }.lottery-action-icon { display:grid; width:33px; height:33px; flex:0 0 auto; place-items:center; border-radius:8px; }.lottery-action-icon--blue { background:#ddecff; color:#1677ff; }.lottery-action-icon--green { background:#e2f7ef; color:#16a87b; }.lottery-action-icon--amber { background:#fff0dc; color:#df8c25; }.lottery-action-card h3 { color:#29405f; font-size:14px; font-weight:750; }.lottery-action-card p { margin-top:12px; color:#7d8da5; font-size:12px; line-height:1.55; }.lottery-action-card button { display:inline-flex; align-items:center; gap:2px; margin-top:11px; border:0; background:transparent; color:#1677ff; padding:0; font-size:12px; font-weight:750; }.lottery-action-card button:disabled { color:#94a3b8; cursor:not-allowed; }.lottery-invite-heading { position:relative; }.lottery-action-card .lottery-invite-help-button { display:grid; width:20px; height:20px; flex:0 0 auto; place-items:center; margin:0 0 0 -4px; border-radius:50%; color:#75a38f; font-size:13px; font-weight:800; line-height:1; transition:color .2s ease,background .2s ease; }.lottery-action-card .lottery-invite-help-button:hover,.lottery-action-card .lottery-invite-help-button:focus-visible { background:#d7f1e5; color:#168a63; outline:0; }.lottery-invite-requirement { position:absolute; z-index:10; top:calc(100% + 8px); left:43px; width:244px; border:1px solid #bfe4d3; border-radius:8px; background:#fff; box-shadow:0 12px 24px rgba(31,92,68,.14); color:#48715f; padding:9px 10px; font-size:11px; font-weight:500; line-height:1.55; }
.lottery-ready-panel { background:linear-gradient(145deg,#fff 0%,#f4f9ff 100%); }.lottery-primary-button,.lottery-secondary-button { display:flex; width:100%; min-height:43px; align-items:center; justify-content:flex-start; gap:9px; border:0; border-radius:8px; padding:0 14px; font-size:13px; font-weight:750; text-align:left; transition:.2s ease; }.lottery-primary-button > span,.lottery-secondary-button > span { flex:1; }.lottery-primary-button { background:#1677ff; color:#fff; box-shadow:0 8px 18px rgba(22,119,255,.16); }.lottery-primary-button:not(:disabled):hover { background:#0958d9; box-shadow:0 10px 22px rgba(22,119,255,.23); }.lottery-secondary-button { background:#eaf4ff; color:#1677ff; }.lottery-purchase-remaining { flex:0 0 auto; color:#587aa6; font-size:11px; font-weight:700; white-space:nowrap; }.lottery-secondary-button:not(:disabled):hover { background:#dceeff; }.lottery-primary-button:disabled,.lottery-secondary-button:disabled { cursor:not-allowed; opacity:.55; box-shadow:none; }
.lottery-history-icon { display:grid; width:34px; height:34px; place-items:center; border-radius:9px; background:#e8f2ff; color:#1677ff; }.lottery-history-icon.is-empty { background:#f1f5f9; color:#94a3b8; }.lottery-section-label { color:#58708f; font-size:12px; font-weight:750; }.lottery-probability-list { border-top:1px solid #e2ebf5; }.lottery-probability-item { display:flex; min-height:36px; align-items:center; justify-content:space-between; gap:16px; border-bottom:1px solid #e7eff8; color:#536884; font-size:13px; }.lottery-probability-item strong { color:#1677ff; font-size:12px; font-variant-numeric:tabular-nums; }.lottery-info-column { min-width:0; padding-left:28px; border-left:1px solid #d8e7f6; }.lottery-guarantee-steps { display:grid; grid-template-columns:repeat(5,minmax(0,1fr)); position:relative; max-width:420px; }.lottery-guarantee-steps::before { position:absolute; top:14px; right:10%; left:10%; height:1px; background:#d5e4f5; content:''; }.lottery-guarantee-step { position:relative; z-index:1; display:grid; justify-items:center; gap:6px; color:#8898ac; font-size:10px; }.lottery-guarantee-step strong { display:grid; width:29px; height:29px; place-items:center; border:1px solid #d3dfec; border-radius:50%; background:#fff; color:#68809e; font-size:12px; font-variant-numeric:tabular-nums; }.lottery-guarantee-step.is-final { color:#1677ff; font-weight:750; }.lottery-guarantee-step.is-final strong { border-color:#1677ff; background:#1677ff; color:#fff; box-shadow:0 5px 12px rgba(22,119,255,.22); }.lottery-guarantee-copy { max-width:520px; margin-top:17px; color:#71819a; font-size:12px; line-height:1.65; }.lottery-activity-rules { margin-top:22px; padding-top:18px; border-top:1px solid #e2ebf5; }.lottery-activity-rules ul { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:12px 20px; margin-top:11px; }.lottery-activity-rules li { position:relative; padding-left:11px; color:#71819a; font-size:11px; line-height:1.65; }.lottery-activity-rules li::before { position:absolute; top:.47rem; left:0; width:4px; height:4px; border-radius:50%; background:#8ebbf4; content:''; }.lottery-result-mark { display:grid; width:76px; height:76px; place-items:center; margin:auto; border-radius:24px; background:#e6f2ff; color:#1677ff; box-shadow:0 12px 28px rgba(22,119,255,.18); }.lottery-result-mark.is-empty { background:#f1f5f9; color:#94a3b8; box-shadow:none; }.lottery-slide-enter-active,.lottery-slide-leave-active { transition:.22s ease; }.lottery-slide-enter-from,.lottery-slide-leave-to { opacity:0; transform:translateY(-8px); }
.wheel-pointer { border-top:31px solid #fff; border-bottom:0; }
.lottery-wheel-action-label { display:block; font-size:14px; font-weight:750; text-shadow:0 1px rgba(7,62,152,.25); animation:lottery-action-copy 2.4s ease-in-out infinite; }
.lottery-wheel-action { display:flex; flex-direction:column; gap:1px; border:1px solid rgba(255,255,255,.62); background:linear-gradient(145deg,#5cadff 0%,#1677ff 48%,#0958d9 100%); box-shadow:inset 0 2px 1px rgba(255,255,255,.5),inset 0 -5px 8px rgba(4,62,156,.34),0 6px 0 #074ba8,0 13px 20px rgba(11,78,182,.34); transition:filter .2s ease,box-shadow .2s ease; animation:lottery-action-3d-breathe 2.8s ease-in-out infinite; }
.lottery-wheel-action::after { position:absolute; inset:6px; border:1px solid rgba(255,255,255,.2); border-radius:50%; box-shadow:inset 0 0 10px rgba(255,255,255,.12); content:''; pointer-events:none; }
.lottery-wheel-action:not(:disabled):hover { background:linear-gradient(145deg,#69b4ff 0%,#1677ff 48%,#0958d9 100%); filter:brightness(1.07) saturate(1.08); transform:perspective(280px) rotateX(7deg) scale(1.04); animation-play-state:paused; }
.lottery-wheel-action:not(:disabled):active { box-shadow:inset 0 3px 7px rgba(4,62,156,.42),0 2px 0 #074ba8,0 7px 13px rgba(11,78,182,.3); transform:perspective(280px) rotateX(7deg) translateY(4px) scale(.99); }
.lottery-wheel-action-label,.lottery-wheel-action small { position:relative; z-index:1; line-height:1; }.lottery-wheel-action small { margin-top:0; }
.lottery-wheel::before { width:86px; height:86px; border:0; background:transparent; box-shadow:none; }
.lottery-wheel-action { align-items:center; justify-content:center; text-align:center; }
.lottery-wheel-action-label,.lottery-wheel-action small { width:100%; text-align:center; }
.wheel-label.is-quota { color:#1260bd; }.wheel-label.is-voucher { color:#7c3aa8; }.wheel-label.is-none { color:#728096; }.wheel-label.is-jackpot { width:76px; min-height:26px; border:1px solid rgba(182,111,0,.22); border-radius:7px; background:rgba(255,244,195,.78); color:#ad6800; font-size:13px; text-shadow:0 1px rgba(255,255,255,.8); }
.lottery-probability-item:last-child { border-bottom:0; }.lottery-probability-item.is-quota span,.lottery-probability-item.is-quota strong,.lottery-history-icon.is-quota,.lottery-history-icon + div .is-quota,.lottery-result-mark.is-quota,.lottery-result-mark + .is-quota { color:#1260bd; }.lottery-probability-item.is-voucher span,.lottery-probability-item.is-voucher strong,.lottery-history-icon.is-voucher,.lottery-history-icon + div .is-voucher,.lottery-result-mark.is-voucher,.lottery-result-mark + .is-voucher { color:#7c3aa8; }.lottery-probability-item.is-none span,.lottery-probability-item.is-none strong,.lottery-history-icon.is-none,.lottery-history-icon + div .is-none,.lottery-result-mark.is-none,.lottery-result-mark + .is-none { color:#728096; }.lottery-probability-item.is-jackpot span,.lottery-probability-item.is-jackpot strong,.lottery-history-icon.is-jackpot,.lottery-history-icon + div .is-jackpot,.lottery-result-mark.is-jackpot,.lottery-result-mark + .is-jackpot { color:#ad6800; }.lottery-history-icon.is-quota,.lottery-result-mark.is-quota { background:#e6f1ff; box-shadow:0 12px 28px rgba(18,96,189,.16); }.lottery-history-icon.is-voucher,.lottery-result-mark.is-voucher { background:#f3e8ff; box-shadow:0 12px 28px rgba(124,58,168,.15); }.lottery-history-icon.is-none,.lottery-result-mark.is-none { background:#f1f5f9; box-shadow:none; }.lottery-history-icon.is-jackpot,.lottery-result-mark.is-jackpot { background:#fff1c9; box-shadow:0 12px 28px rgba(194,128,0,.2); }
.wheel-label.is-voucher { width:92px; min-height:28px; border:0; border-radius:0; background:transparent; box-shadow:none; color:#7c3aa8; font-size:12px; font-weight:800; text-shadow:0 0 8px rgba(124,58,168,.16); }.wheel-label.is-jackpot { width:92px; min-height:28px; border:0; border-radius:0; background:transparent; box-shadow:none; color:#c27a00; font-size:14px; font-weight:850; text-shadow:0 0 9px rgba(238,177,30,.34),0 1px rgba(255,255,255,.9); }
.lottery-purchase-summary { overflow:hidden; border:1px solid #dbe8f6; border-radius:10px; background:#f8fbff; }.lottery-purchase-summary div { display:flex; align-items:center; justify-content:space-between; gap:16px; padding:13px 14px; }.lottery-purchase-summary div + div { border-top:1px solid #e6eef8; }.lottery-purchase-summary span { color:#71819a; font-size:13px; }.lottery-purchase-summary strong { color:#1677ff; font-size:15px; font-weight:800; }.dark .lottery-purchase-summary { border-color:rgba(76,106,145,.5); background:#101f34; }.dark .lottery-purchase-summary div + div { border-color:rgba(76,106,145,.35); }
.lottery-wheel-action.is-drawing .lottery-wheel-action-label { animation-duration:.46s; }
.lottery-recent-panel { display:flex; flex-direction:column; }.lottery-recent-list { border-top:1px solid #e2ebf5; }.lottery-recent-item { display:flex; min-height:54px; align-items:center; gap:12px; border-bottom:1px solid #e7eff8; padding:9px 0; }.lottery-recent-item:last-child { border-bottom:0; }.lottery-recent-empty { display:flex; flex:1; min-height:170px; flex-direction:column; align-items:center; justify-content:center; gap:9px; color:#9aacca; font-size:13px; }.lottery-recent-empty svg { color:#8ebbf4; }.lottery-recent-footer { margin-top:auto; border-top:1px solid #e7eff8; padding-top:13px; color:#8a9bb2; font-size:11px; }
.lottery-probability-notes { margin-top:16px; border-top:1px solid #e7eff8; padding-top:13px; }.lottery-probability-notes p { color:#71819a; font-size:11px; font-weight:750; }.lottery-probability-notes ul { display:grid; gap:5px; margin-top:7px; }.lottery-probability-notes li { position:relative; padding-left:10px; color:#8a9bb2; font-size:10px; line-height:1.55; }.lottery-probability-notes li::before { position:absolute; top:.42rem; left:0; width:3px; height:3px; border-radius:50%; background:#8ebbf4; content:''; }
.lottery-result-balance-change { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:0; margin-top:20px; overflow:hidden; border:1px solid #dbe8f6; border-radius:10px; background:#f8fbff; text-align:left; }.lottery-result-balance-change div { min-width:0; padding:12px 10px; }.lottery-result-balance-change div + div { border-left:1px solid #e4edf7; }.lottery-result-balance-change span { display:block; color:#71819a; font-size:11px; }.lottery-result-balance-change strong { display:block; margin-top:5px; overflow:hidden; color:#1677ff; font-size:13px; font-variant-numeric:tabular-nums; text-overflow:ellipsis; white-space:nowrap; }.lottery-result-balance-change .is-reward strong { color:#169b6b; }.dark .lottery-result-balance-change { border-color:rgba(76,106,145,.5); background:#101f34; }.dark .lottery-result-balance-change div + div { border-color:rgba(76,106,145,.35); }.dark .lottery-result-balance-change span { color:#9aacca; }.dark .lottery-result-balance-change strong { color:#86bfff; }.dark .lottery-result-balance-change .is-reward strong { color:#62d3a6; }
@keyframes lottery-pointer-breathe { 0%,100% { filter:drop-shadow(0 3px 5px rgba(19,76,145,.22)); transform:translateY(0); } 50% { filter:drop-shadow(0 5px 9px rgba(19,76,145,.34)); transform:translateY(2px); } }
@keyframes lottery-halo-breathe { 0%,100% { opacity:.6; transform:scale(.97); } 50% { opacity:1; transform:scale(1.035); } }
@keyframes lottery-jackpot-glow { 0%,100% { opacity:.52; transform:scale(.98); } 50% { opacity:.96; transform:scale(1.045); } }
@keyframes lottery-spark-drift { 0%,100% { opacity:.3; transform:translate3d(0,0,0) scale(.65); } 50% { opacity:1; transform:translate3d(0,-8px,0) scale(1); } }
@keyframes lottery-action-breathe { 0%,100% { box-shadow:0 9px 18px rgba(22,119,255,.27); transform:scale(1); } 50% { box-shadow:0 12px 25px rgba(255,255,255,.28),0 12px 25px rgba(22,119,255,.35); transform:scale(1.035); } }
@keyframes lottery-action-3d-breathe { 0%,100% { box-shadow:inset 0 2px 1px rgba(255,255,255,.5),inset 0 -5px 8px rgba(4,62,156,.34),0 6px 0 #074ba8,0 13px 20px rgba(11,78,182,.34); transform:perspective(280px) rotateX(7deg) scale(1); } 50% { box-shadow:inset 0 2px 1px rgba(255,255,255,.58),inset 0 -5px 8px rgba(4,62,156,.3),0 7px 0 #074ba8,0 17px 26px rgba(11,78,182,.42); transform:perspective(280px) rotateX(7deg) scale(1.035); } }
@keyframes lottery-action-copy { 0%,100% { letter-spacing:0; opacity:1; transform:translateY(0); } 50% { letter-spacing:.06em; opacity:.86; transform:translateY(-1px); } }
@keyframes lottery-broadcast-message-cycle { 0% { opacity:0; transform:translateY(14px); } 14% { opacity:1; transform:translateY(0); } 72% { opacity:1; transform:translateY(0); } 100% { opacity:0; transform:translateY(-14px); } }
@keyframes lottery-broadcast-field-in { from { opacity:0; transform:translateY(5px); } to { opacity:1; transform:translateY(0); } }
@media (max-width: 860px) { .lottery-hero { grid-template-columns:1fr; }.lottery-hero-copy { padding:32px 28px 10px; }.lottery-wheel-wrap { min-height:340px; }.wheel-pointer { top:8px; } }
@media (max-width: 1023px) { .lottery-activity-rules ul { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@media (max-width: 480px) { .lottery-hero { border-radius:12px; }.lottery-hero-copy { padding:27px 20px 5px; }.lottery-hero h2 { font-size:27px; }.lottery-stat-grid { gap:8px; }.lottery-hero-stat { padding:12px; }.lottery-wheel { width:270px; height:270px; }.wheel-label { font-size:11px; }.lottery-wheel-wrap { min-height:302px; padding:18px; }.lottery-history-button { width:34px; min-height:34px; padding:0; }.lottery-history-button span { display:none; }.lottery-activity-rules ul { grid-template-columns:1fr; gap:5px; }.lottery-broadcast { grid-template-columns:minmax(0,1fr); grid-template-rows:auto auto; row-gap:6px; padding:9px 10px; }.lottery-broadcast-panel { grid-template-columns:minmax(0,1fr); gap:7px; }.lottery-broadcast-divider { width:100%; height:1px; }.lottery-broadcast-content { grid-template-columns:minmax(0,1fr); grid-template-areas:'message' 'meta'; gap:7px; }.lottery-broadcast-meta { width:100%; min-width:0; grid-template-columns:minmax(0,1fr) 42px; gap:10px; border-top:1px solid #e5ebf3; padding-top:5px; padding-left:0; }.lottery-broadcast-message { flex-wrap:wrap; gap:5px; }.lottery-broadcast-prize { max-width:42vw; }.lottery-broadcast-value-label { font-size:10px; }.lottery-broadcast-value { font-size:12px; } }
@media (prefers-reduced-motion: reduce) { .wheel-pointer,.lottery-wheel-wrap::before,.wheel-spark,.lottery-wheel-action { animation:none; }.lottery-wheel { transition-duration:.01ms; } }
@media (prefers-reduced-motion: reduce) { .lottery-wheel-action-label { animation:none; } }
@media (prefers-reduced-motion: reduce) { .lottery-broadcast-dynamic { animation:none; } }
</style>

<style>
.dark .lottery-page { --lottery-ink:#edf4ff; --lottery-line:rgba(76,106,145,.45); }.dark .lottery-history-button,.dark .lottery-panel { border-color:var(--lottery-line); background:#0e192b; }.dark .lottery-history-button { color:#b3c3da; }.dark .lottery-history-button:hover { border-color:#3d82dc; background:#12233d; color:#cfe5ff; }.dark .lottery-ready-panel { background:linear-gradient(145deg,#0f1c30 0%,#122640 100%); }.dark .lottery-purchase-remaining { color:#a9c6e8; }.dark .lottery-action-card { border-color:rgba(76,106,145,.5); background:#101f34; }.dark .lottery-action-card h3 { color:#edf4ff; }.dark .lottery-action-card p,.dark .lottery-panel-description,.dark .lottery-activity-rules li,.dark .lottery-probability-notes li { color:#9aacca; }.dark .lottery-probability-grid { border-color:rgba(76,106,145,.5); background:#101f34; }.dark .lottery-probability-item,.dark .lottery-info-column,.dark .lottery-activity-rules,.dark .lottery-recent-list,.dark .lottery-recent-item,.dark .lottery-recent-footer,.dark .lottery-probability-notes { border-color:rgba(76,106,145,.35); color:#c8d6e9; }.dark .lottery-action-card .lottery-invite-help-button { color:#7fc9a7; }.dark .lottery-action-card .lottery-invite-help-button:hover,.dark .lottery-action-card .lottery-invite-help-button:focus-visible { background:rgba(46,161,112,.18); color:#b7f0d2; }.dark .lottery-invite-requirement { border-color:rgba(78,163,119,.42); background:#122a26; color:#c3e4d3; box-shadow:0 12px 24px rgba(0,0,0,.28); }
.dark .lottery-broadcast { border-color:rgba(76,106,145,.5); background:#0e192b; }.dark .lottery-broadcast-label,.dark .lottery-broadcast--jackpot .lottery-broadcast-label { color:#f6d477; }.dark .lottery-broadcast-user { color:#edf4ff; }.dark .lottery-broadcast-copy,.dark .lottery-broadcast-subtitle,.dark .lottery-broadcast-time { color:#9aacca; }.dark .lottery-broadcast-probability { color:#f6d477; }
</style>
