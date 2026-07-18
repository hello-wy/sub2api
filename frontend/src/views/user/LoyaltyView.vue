<template>
  <AppLayout>
    <div class="loyalty-shell -mx-4 -my-6 min-h-[calc(100vh-4rem)] overflow-hidden px-4 py-6 sm:-mx-6 sm:py-7 lg:-mx-8 lg:px-8">
      <div class="relative mx-auto max-w-[1180px] space-y-4">
        <section class="loyalty-hero loyalty-panel p-5 sm:p-6 lg:p-7">
          <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_230px] lg:items-center">
            <div class="flex min-w-0 flex-col gap-5 sm:flex-row sm:items-center">
              <div class="loyalty-mascot mx-auto sm:mx-0">
                <div class="loyalty-mascot-inner" aria-hidden="true">
                  <Icon name="gift" size="xl" :stroke-width="1.8" />
                </div>
              </div>

              <div class="min-w-0 text-center sm:text-left">
                <h2 class="text-3xl font-black tracking-normal text-slate-950 dark:text-white">
                  {{ t('loyalty.title') }}
                </h2>
                <p class="mt-2 max-w-3xl text-base font-medium leading-7 text-slate-600 dark:text-dark-300">
                  {{ t('loyalty.description') }}
                </p>
              </div>
            </div>

            <div class="flex flex-col items-center gap-3 lg:items-stretch">
              <button class="loyalty-recharge-btn" type="button" @click="goRecharge">
                <Icon name="bolt" size="md" :stroke-width="2.2" />
                <span>{{ t('loyalty.rechargeNow') }}</span>
              </button>
              <p class="text-center text-sm font-medium text-slate-500 dark:text-dark-400">
                {{ t('loyalty.rechargeHint') }}
              </p>
            </div>
          </div>
        </section>

        <div v-if="loading" class="loyalty-panel flex justify-center py-14">
          <LoadingSpinner size="lg" />
        </div>

        <template v-else>
          <div v-if="!pointsDefinition" class="loyalty-panel loyalty-alert-panel p-5">
            <div class="flex items-start gap-3">
              <div class="loyalty-alert-icon">
                <Icon name="exclamationTriangle" size="md" />
              </div>
              <div>
                <p class="font-semibold text-amber-900 dark:text-amber-100">{{ t('loyalty.attributeMissingTitle') }}</p>
                <p class="mt-1 text-sm text-amber-800 dark:text-amber-200">{{ t('loyalty.attributeMissingDesc') }}</p>
              </div>
            </div>
          </div>

          <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <article
              v-for="card in statCards"
              :key="card.label"
              class="loyalty-stat-card"
            >
              <div class="flex items-start gap-4">
                <div class="loyalty-stat-icon">
                  <Icon :name="card.icon" size="lg" :stroke-width="1.9" />
                </div>
                <div class="min-w-0">
                  <p class="text-sm font-bold text-slate-500 dark:text-dark-300">{{ card.label }}</p>
                  <p class="mt-2 break-words text-3xl font-black tracking-normal text-slate-950 dark:text-white">
                    {{ card.value }}
                  </p>
                  <p class="mt-1 text-xs font-semibold text-slate-500 dark:text-dark-400">{{ card.hint }}</p>
                </div>
              </div>
            </article>
          </section>

          <section class="grid gap-5 xl:grid-cols-2">
            <article
              v-for="plan in planCards"
              :key="plan.key"
              class="loyalty-panel p-5 sm:p-6"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="flex min-w-0 flex-1 items-start gap-3">
                  <div class="loyalty-section-icon">
                    <Icon :name="plan.icon" size="md" />
                  </div>
                  <div class="min-w-0">
                    <h3 class="text-lg font-black tracking-normal text-slate-950 dark:text-white">{{ plan.title }}</h3>
                    <p class="mt-1 text-sm font-medium leading-6 text-slate-500 dark:text-dark-400">{{ plan.hint }}</p>
                  </div>
                </div>
                <span class="loyalty-plan-badge shrink-0">{{ plan.badge }}</span>
              </div>

              <div
                class="mt-5 grid gap-3 sm:grid-cols-2"
                :class="plan.tiers.length === 3 ? 'lg:grid-cols-3' : 'lg:grid-cols-4'"
              >
                <div
                  v-for="tier in plan.tiers"
                  :key="`${plan.key}-${tier.rule.level}`"
                  class="loyalty-tier-tile"
                  :class="tierTileClass(tier.state)"
                >
                  <div class="loyalty-tier-main">
                    <p class="loyalty-tier-level">{{ tier.rule.level }}</p>
                    <p class="loyalty-tier-condition">
                      {{ ruleCondition(tier.rule) }}
                    </p>
                  </div>
                  <span class="loyalty-discount-pill">
                    {{ t('loyalty.ruleDiscount', { discount: tier.rule.discount }) }}
                  </span>
                </div>
              </div>
            </article>
          </section>

          <section class="loyalty-panel loyalty-rules-panel p-5 sm:p-6">
            <div>
              <h3 class="text-lg font-black tracking-normal text-slate-950 dark:text-white">{{ t('loyalty.rulesTitle') }}</h3>
            </div>
            <ul class="loyalty-rules-list mt-4">
              <li
                v-for="item in ruleNotes"
                :key="item.title"
                class="loyalty-rule-item"
              >
                <div class="min-w-0">
                  <h4 class="text-base font-black tracking-normal text-slate-950 dark:text-white">{{ item.title }}</h4>
                  <p class="mt-1 text-sm font-medium leading-6 text-slate-500 dark:text-dark-400">{{ item.description }}</p>
                </div>
              </li>
            </ul>
          </section>

          <section class="loyalty-panel overflow-hidden p-5 sm:p-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-lg font-black tracking-normal text-slate-950 dark:text-white">{{ t('loyalty.detailsTitle') }}</h3>
                <p class="mt-1 text-sm font-medium text-slate-500 dark:text-dark-400">{{ t('loyalty.detailsHint') }}</p>
              </div>
              <span class="loyalty-table-action">
                <Icon name="book" size="sm" />
                <span>{{ t('loyalty.viewRules') }}</span>
              </span>
            </div>

            <div class="mt-4 overflow-x-auto">
              <table class="loyalty-table">
                <thead>
                  <tr>
                    <th>{{ t('loyalty.tablePlan') }}</th>
                    <th>{{ t('loyalty.tableLevel') }}</th>
                    <th>{{ t('loyalty.tableCondition') }}</th>
                    <th>{{ t('loyalty.tableDiscount') }}</th>
                    <th class="text-right">{{ t('loyalty.tableStatus') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="row in benefitRows"
                    :key="`${row.scope}-${row.level}`"
                    :class="statusClass(row.state)"
                  >
                    <td v-if="row.showPlan" :rowspan="row.rowSpan" class="min-w-[150px]">
                      <div class="flex items-center gap-3">
                        <div class="loyalty-table-plan-icon">
                          <Icon :name="row.planIcon" size="md" />
                        </div>
                        <div>
                          <p class="font-bold text-slate-800 dark:text-dark-100">{{ row.plan }}</p>
                          <p class="text-xs font-semibold text-slate-500 dark:text-dark-400">{{ row.planHint }}</p>
                        </div>
                      </div>
                    </td>
                    <td class="font-semibold text-slate-600 dark:text-dark-300">{{ row.level }}</td>
                    <td class="font-semibold text-slate-600 dark:text-dark-300">{{ row.condition }}</td>
                    <td class="text-base font-black text-primary-600 dark:text-primary-300">{{ row.discount }}</td>
                    <td class="text-right">
                      <span class="loyalty-status-pill" :class="statusClass(row.state)">
                        {{ statusLabel(row.state) }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </template>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { paymentAPI, userAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  findLoyaltyPointsDefinitions,
  formatLoyaltyPoints,
  permanentLoyaltyRules,
  readLoyaltyPoints,
  resolveLoyaltyProgress,
  weeklyLoyaltyRules,
  type LoyaltyProgress,
  type LoyaltyRule,
} from '@/utils/loyalty'
import type { UserAttributeDefinition, UserAttributeValue } from '@/types'
import type { PaymentLoyaltyInfo, PaymentLoyaltyRule } from '@/types/payment'

type IconName =
  | 'badge'
  | 'bolt'
  | 'book'
  | 'calendar'
  | 'chart'
  | 'database'
  | 'dollar'
  | 'gift'
  | 'link'
  | 'shield'
  | 'trendingUp'
  | 'users'

type RuleState = 'current' | 'unlocked' | 'locked'

interface TierCard {
  rule: LoyaltyRule
  state: RuleState
}

interface PlanCard {
  key: LoyaltyRule['scope']
  title: string
  hint: string
  badge: string
  icon: IconName
  tiers: TierCard[]
}

interface BenefitRow {
  scope: LoyaltyRule['scope']
  showPlan: boolean
  rowSpan: number
  plan: string
  planHint: string
  planIcon: IconName
  level: string
  condition: string
  discount: string
  state: RuleState
}

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(true)
const definitions = ref<UserAttributeDefinition[]>([])
const values = ref<UserAttributeValue[]>([])
const paymentLoyalty = ref<PaymentLoyaltyInfo | null>(null)

const weeklyRules = computed(() => normalizeRuleSettings(paymentLoyalty.value?.weekly_rules, weeklyLoyaltyRules, 'weekly'))
const permanentRules = computed(() => normalizeRuleSettings(paymentLoyalty.value?.permanent_rules, permanentLoyaltyRules, 'permanent'))

const pointDefinitions = computed(() => findLoyaltyPointsDefinitions(definitions.value))
const pointsDefinition = computed(() => Boolean(pointDefinitions.value.weekly && pointDefinitions.value.permanent))
const weeklyPoints = computed(() => readLoyaltyPoints(definitions.value, values.value, 'weekly'))
const permanentPoints = computed(() => readLoyaltyPoints(definitions.value, values.value, 'permanent'))
const weeklyProgress = computed(() => resolveLoyaltyProgress(weeklyPoints.value, weeklyRules.value))
const permanentProgress = computed(() => resolveLoyaltyProgress(permanentPoints.value, permanentRules.value))
const bestDiscount = computed(() => Math.max(
  weeklyProgress.value.current?.discount ?? 0,
  permanentProgress.value.current?.discount ?? 0,
))

const bestCurrentRule = computed<LoyaltyRule | null>(() => {
  return [weeklyProgress.value.current, permanentProgress.value.current]
    .filter((rule): rule is LoyaltyRule => Boolean(rule))
    .sort((a, b) => b.discount - a.discount || b.points - a.points)[0] ?? null
})

const statCards = computed(() => [
  {
    label: t('loyalty.weeklyPoints'),
    value: formatLoyaltyPoints(weeklyPoints.value),
    hint: t('loyalty.weeklyPointsHint'),
    icon: 'database' as IconName,
  },
  {
    label: t('loyalty.permanentPoints'),
    value: formatLoyaltyPoints(permanentPoints.value),
    hint: t('loyalty.permanentPointsHint'),
    icon: 'trendingUp' as IconName,
  },
  {
    label: t('loyalty.currentLevel'),
    value: bestCurrentRule.value?.level ?? t('loyalty.noTier'),
    hint: bestCurrentRule.value
      ? t('loyalty.unlockedDiscount', { discount: bestCurrentRule.value.discount })
      : t('loyalty.noTierHint'),
    icon: 'shield' as IconName,
  },
  {
    label: t('loyalty.highestDiscount'),
    value: `${bestDiscount.value}%`,
    hint: t('loyalty.bestDiscountHint'),
    icon: 'dollar' as IconName,
  },
])

const planCards = computed<PlanCard[]>(() => [
  {
    key: 'weekly',
    title: t('loyalty.weeklyPlan'),
    hint: t('loyalty.weeklyHint'),
    badge: t('loyalty.weeklyBadge'),
    icon: 'calendar',
    tiers: buildTierCards(weeklyRules.value, weeklyProgress.value, weeklyPoints.value),
  },
  {
    key: 'permanent',
    title: t('loyalty.permanentPlan'),
    hint: t('loyalty.permanentHint'),
    badge: t('loyalty.permanentBadge'),
    icon: 'link',
    tiers: buildTierCards(permanentRules.value, permanentProgress.value, permanentPoints.value),
  },
])

const ruleNotes = computed(() => [
  {
    title: t('loyalty.ruleHigherTitle'),
    description: t('loyalty.ruleHigherDesc'),
  },
  {
    title: t('loyalty.weeklyResetTitle'),
    description: t('loyalty.weeklyResetDesc'),
  },
  {
    title: t('loyalty.permanentStableTitle'),
    description: t('loyalty.permanentStableDesc'),
  },
  {
    title: t('loyalty.earnTitle'),
    description: t('loyalty.earnRechargeDesc'),
  },
  {
    title: t('loyalty.bonusTitle'),
    description: t('loyalty.bonusDesc'),
  },
])

const benefitRows = computed<BenefitRow[]>(() => [
  ...buildBenefitRows('weekly', weeklyRules.value, weeklyProgress.value, weeklyPoints.value),
  ...buildBenefitRows('permanent', permanentRules.value, permanentProgress.value, permanentPoints.value),
])

function normalizeRuleSettings(
  source: PaymentLoyaltyRule[] | undefined,
  fallback: LoyaltyRule[],
  scope: LoyaltyRule['scope'],
): LoyaltyRule[] {
  if (!Array.isArray(source) || source.length === 0) return fallback
  const rules = source
    .map((rule, index) => ({
      scope,
      level: rule.level || `L${index + 1}`,
      points: Number(rule.points) || 0,
      discount: Number(rule.discount) || 0,
    }))
    .filter((rule) => rule.points > 0 && rule.discount >= 0)
    .sort((a, b) => a.points - b.points)
  return rules.length > 0 ? rules : fallback
}

function buildTierCards(rules: LoyaltyRule[], progress: LoyaltyProgress, points: number): TierCard[] {
  return rules.map((rule) => ({
    rule,
    state: ruleState(rule, progress, points),
  }))
}

function buildBenefitRows(
  scope: LoyaltyRule['scope'],
  rules: LoyaltyRule[],
  progress: LoyaltyProgress,
  points: number,
): BenefitRow[] {
  return rules.map((rule, index) => ({
    scope,
    showPlan: index === 0,
    rowSpan: rules.length,
    plan: scope === 'weekly' ? t('loyalty.weeklyPlanShort') : t('loyalty.permanentPlanShort'),
    planHint: scope === 'weekly' ? t('loyalty.weeklyBadge') : t('loyalty.permanentBadge'),
    planIcon: scope === 'weekly' ? 'calendar' : 'link',
    level: rule.level,
    condition: ruleCondition(rule),
    discount: `${rule.discount}%`,
    state: ruleState(rule, progress, points),
  }))
}

function ruleCondition(rule: LoyaltyRule): string {
  if (rule.scope === 'permanent') {
    return t('loyalty.unlockAtCurrency', { amount: formatLoyaltyPoints(rule.points) })
  }
  return t('loyalty.unlockAt', { points: formatLoyaltyPoints(rule.points) })
}

function ruleState(rule: LoyaltyRule, progress: LoyaltyProgress, points: number): RuleState {
  if (progress.current?.points === rule.points) return 'current'
  if (points >= rule.points) return 'unlocked'
  return 'locked'
}

function tierTileClass(state: RuleState): string {
  return `is-${state}`
}

function statusClass(state: RuleState): string {
  return `is-${state}`
}

function statusLabel(state: RuleState): string {
  if (state === 'current') return t('loyalty.statusCurrent')
  if (state === 'unlocked') return t('loyalty.statusUnlocked')
  return t('loyalty.statusPending')
}

async function loadAttributes(): Promise<void> {
  loading.value = true
  try {
    const [resp, checkoutResp] = await Promise.all([
      userAPI.getMyAttributes(),
      paymentAPI.getCheckoutInfo().catch(() => null),
    ])
    definitions.value = resp.definitions
    values.value = resp.values
    paymentLoyalty.value = checkoutResp?.data.loyalty ?? null
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

function goRecharge(): void {
  void router.push({ path: '/wallet', query: { tab: 'recharge' } })
}

onMounted(() => {
  void loadAttributes()
})
</script>

<style scoped>
.loyalty-shell {
  position: relative;
}

.loyalty-panel {
  border: 1px solid rgba(145, 202, 255, 0.32);
  border-radius: 0.5rem;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 12px 30px rgba(47, 94, 143, 0.08);
}

.loyalty-hero {
  min-height: 8rem;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.065);
}

.loyalty-alert-panel {
  border-color: rgba(245, 158, 11, 0.38);
  background: rgba(255, 251, 235, 0.94);
  box-shadow: 0 10px 30px rgba(146, 64, 14, 0.08);
}

.loyalty-mascot {
  position: relative;
  display: flex;
  height: 6rem;
  width: 6rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  border: 1px solid rgba(145, 202, 255, 0.72);
  background: rgba(230, 244, 255, 0.88);
  box-shadow: 0 12px 26px rgba(22, 119, 255, 0.14);
}

.loyalty-mascot-inner {
  display: flex;
  height: 3.75rem;
  width: 3.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  background: linear-gradient(135deg, #4096ff, #1677ff);
  color: white;
  box-shadow: 0 10px 20px rgba(22, 119, 255, 0.24);
}

.loyalty-recharge-btn {
  display: inline-flex;
  min-height: 3.5rem;
  align-items: center;
  justify-content: center;
  gap: 0.625rem;
  border-radius: 0.5rem;
  background: linear-gradient(135deg, #4096ff 0%, #1677ff 58%, #0958d9 100%);
  padding: 0 1.5rem;
  font-size: 1rem;
  font-weight: 900;
  color: white;
  box-shadow: 0 12px 26px rgba(22, 119, 255, 0.26);
  transition: transform 160ms ease, box-shadow 160ms ease;
}

.loyalty-recharge-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 16px 32px rgba(22, 119, 255, 0.32);
}

.loyalty-recharge-btn:focus-visible {
  outline: 3px solid rgba(22, 119, 255, 0.28);
  outline-offset: 3px;
}

.loyalty-alert-icon,
.loyalty-section-icon,
.loyalty-table-plan-icon,
.loyalty-stat-icon {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
}

.loyalty-alert-icon {
  height: 2.5rem;
  width: 2.5rem;
  background: white;
  color: #d97706;
  box-shadow: inset 0 0 0 1px rgba(245, 158, 11, 0.22);
}

.loyalty-section-icon,
.loyalty-table-plan-icon {
  height: 2.75rem;
  width: 2.75rem;
  background: rgba(230, 244, 255, 0.88);
  color: #1677ff;
  box-shadow: inset 0 0 0 1px rgba(145, 202, 255, 0.72);
}

.loyalty-stat-card {
  min-height: 7.75rem;
  border: 1px solid rgba(145, 202, 255, 0.32);
  border-radius: 0.5rem;
  background: rgba(255, 255, 255, 0.94);
  padding: 1.25rem;
  box-shadow: 0 10px 26px rgba(47, 94, 143, 0.07);
}

.loyalty-stat-icon {
  height: 3.25rem;
  width: 3.25rem;
  background: linear-gradient(135deg, rgba(230, 244, 255, 0.92), rgba(186, 224, 255, 0.76));
  color: #1677ff;
  box-shadow: inset 0 0 0 1px rgba(145, 202, 255, 0.72);
}

.loyalty-plan-badge {
  width: fit-content;
  border-radius: 9999px;
  background: rgba(248, 250, 252, 0.9);
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 900;
  color: #64748b;
  box-shadow: inset 0 0 0 1px rgba(226, 232, 240, 0.9);
}

.loyalty-tier-tile {
  display: flex;
  min-height: 7rem;
  flex-direction: column;
  align-items: center;
  justify-content: space-between;
  border-radius: 0.5rem;
  border: 1px solid rgba(186, 224, 255, 0.82);
  background: rgba(255, 255, 255, 0.7);
  padding: 0.95rem 0.75rem;
  text-align: center;
}

.loyalty-tier-tile.is-current {
  border-color: rgba(64, 150, 255, 0.86);
  background:
    linear-gradient(180deg, rgba(230, 244, 255, 0.98), rgba(186, 224, 255, 0.72));
  box-shadow:
    inset 0 0 0 1px rgba(22, 119, 255, 0.18),
    0 16px 34px rgba(22, 119, 255, 0.18);
}

.loyalty-tier-tile.is-unlocked {
  border-color: rgba(145, 202, 255, 0.9);
  background: rgba(230, 244, 255, 0.72);
}

.loyalty-tier-tile.is-locked {
  border-color: rgba(226, 232, 240, 0.98);
  background: rgba(248, 250, 252, 0.76);
}

.loyalty-tier-main {
  display: flex;
  min-height: 3.75rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.loyalty-tier-level {
  font-size: 1.125rem;
  font-weight: 950;
  line-height: 1.1;
  color: #0f172a;
}

.loyalty-tier-condition {
  margin-top: 0.6rem;
  font-size: 0.8125rem;
  font-weight: 750;
  line-height: 1.35;
  color: #64748b;
}

.loyalty-tier-tile.is-current .loyalty-tier-level {
  color: #0958d9;
}

.loyalty-tier-tile.is-current .loyalty-tier-condition {
  color: #1677ff;
}

.loyalty-tier-tile.is-current .loyalty-discount-pill {
  background: #1677ff;
  color: white;
  box-shadow: 0 8px 18px rgba(22, 119, 255, 0.24);
}

.loyalty-tier-tile.is-locked .loyalty-tier-level,
.loyalty-tier-tile.is-locked .loyalty-tier-condition {
  color: #94a3b8;
}

.loyalty-tier-tile.is-locked .loyalty-discount-pill {
  background: rgba(241, 245, 249, 0.9);
  color: #94a3b8;
  box-shadow: inset 0 0 0 1px rgba(226, 232, 240, 0.95);
}

.loyalty-discount-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: rgba(230, 244, 255, 0.88);
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 900;
  color: #1677ff;
  box-shadow: inset 0 0 0 1px rgba(145, 202, 255, 0.76);
}

.loyalty-rules-panel {
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.88));
}

.loyalty-rules-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(11.5rem, 1fr));
  gap: 0.75rem;
  list-style: none;
  padding: 0;
}

.loyalty-rule-item {
  position: relative;
  min-height: 7rem;
  border: 1px solid rgba(186, 224, 255, 0.72);
  border-radius: 0.5rem;
  background: rgba(255, 255, 255, 0.62);
  padding: 1rem 1rem 1rem 2.25rem;
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.035);
}

.loyalty-rule-item::before {
  position: absolute;
  top: 1.52rem;
  left: 1rem;
  height: 0.45rem;
  width: 0.45rem;
  border-radius: 9999px;
  background: #1677ff;
  box-shadow: 0 0 0 0.25rem rgba(22, 119, 255, 0.12);
  content: "";
}

.loyalty-table-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  border-radius: 0.5rem;
  background: rgba(248, 250, 252, 0.94);
  padding: 0.5rem 0.75rem;
  font-size: 0.8125rem;
  font-weight: 900;
  color: #64748b;
  box-shadow: inset 0 0 0 1px rgba(226, 232, 240, 0.95);
  transition: background 160ms ease, color 160ms ease;
}

.loyalty-table-action:hover {
  background: rgba(230, 244, 255, 0.9);
  color: #1677ff;
}

.loyalty-table {
  width: 100%;
  min-width: 680px;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.loyalty-table th {
  border-bottom: 1px solid rgba(226, 232, 240, 0.95);
  background: rgba(248, 250, 252, 0.65);
  padding: 0.75rem 1rem;
  text-align: left;
  font-size: 0.75rem;
  font-weight: 900;
  color: #64748b;
}

.loyalty-table td {
  border-bottom: 1px solid rgba(226, 232, 240, 0.78);
  padding: 0.78rem 1rem;
  vertical-align: middle;
}

.loyalty-table tbody tr {
  transition: background 140ms ease;
}

.loyalty-table tbody tr:hover {
  background: rgba(230, 244, 255, 0.42);
}

.loyalty-table tbody tr.is-current {
  background: rgba(230, 244, 255, 0.78);
}

.loyalty-table tbody tr.is-current td {
  border-color: rgba(145, 202, 255, 0.82);
  color: #0958d9;
}

.loyalty-table tbody tr.is-locked td {
  color: #94a3b8;
}

.loyalty-table tbody tr:last-child td {
  border-bottom: 0;
}

.loyalty-status-pill {
  display: inline-flex;
  white-space: nowrap;
  border-radius: 9999px;
  padding: 0.25rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 900;
}

.loyalty-status-pill.is-current {
  background: rgba(186, 224, 255, 0.72);
  color: #0958d9;
  box-shadow: inset 0 0 0 1px rgba(105, 177, 255, 0.72);
}

.loyalty-status-pill.is-unlocked {
  background: rgba(230, 244, 255, 0.86);
  color: #1677ff;
  box-shadow: inset 0 0 0 1px rgba(145, 202, 255, 0.8);
}

.loyalty-status-pill.is-locked {
  background: rgba(241, 245, 249, 0.9);
  color: #94a3b8;
  box-shadow: inset 0 0 0 1px rgba(226, 232, 240, 0.95);
}
</style>

<style>
.dark .loyalty-panel,
.dark .loyalty-stat-card {
  border-color: rgba(71, 85, 105, 0.74);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.92), rgba(17, 24, 39, 0.86));
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.32);
}

.dark .loyalty-hero {
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.94), rgba(0, 62, 179, 0.26)),
    rgba(15, 23, 42, 0.9);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.34);
}

.dark .loyalty-mascot {
  border-color: rgba(64, 150, 255, 0.36);
  background: rgba(0, 44, 140, 0.32);
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.26);
}

.dark .loyalty-mascot-inner {
  background: linear-gradient(135deg, #1677ff, #0958d9);
  box-shadow: 0 12px 24px rgba(22, 119, 255, 0.24);
}

.dark .loyalty-alert-panel {
  border-color: rgba(245, 158, 11, 0.42);
  background: linear-gradient(135deg, rgba(120, 53, 15, 0.34), rgba(15, 23, 42, 0.9));
  box-shadow: 0 14px 34px rgba(0, 0, 0, 0.28);
}

.dark .loyalty-alert-icon {
  background: rgba(120, 53, 15, 0.42);
  color: #fbbf24;
  box-shadow: inset 0 0 0 1px rgba(245, 158, 11, 0.46);
}

.dark .loyalty-plan-badge,
.dark .loyalty-table-action {
  background: rgba(30, 41, 59, 0.7);
  color: #cbd5e1;
  box-shadow: inset 0 0 0 1px rgba(71, 85, 105, 0.82);
}

.dark .loyalty-table-action:hover {
  background: rgba(0, 44, 140, 0.34);
  color: #91caff;
  box-shadow: inset 0 0 0 1px rgba(22, 119, 255, 0.42);
}

.dark .loyalty-section-icon,
.dark .loyalty-table-plan-icon,
.dark .loyalty-stat-icon {
  background: rgba(0, 44, 140, 0.34);
  color: #91caff;
  box-shadow: inset 0 0 0 1px rgba(22, 119, 255, 0.34);
}

.dark .loyalty-tier-tile.is-current {
  border-color: rgba(105, 177, 255, 0.62);
  background: linear-gradient(180deg, rgba(0, 62, 179, 0.48), rgba(15, 23, 42, 0.66));
  box-shadow:
    inset 0 0 0 1px rgba(22, 119, 255, 0.24),
    0 14px 30px rgba(0, 0, 0, 0.22);
}

.dark .loyalty-tier-tile.is-unlocked {
  border-color: rgba(64, 150, 255, 0.4);
  background: rgba(0, 44, 140, 0.24);
}

.dark .loyalty-tier-tile.is-locked {
  border-color: rgba(71, 85, 105, 0.82);
  background: rgba(15, 23, 42, 0.42);
}

.dark .loyalty-discount-pill {
  background: rgba(0, 44, 140, 0.38);
  color: #91caff;
  box-shadow: inset 0 0 0 1px rgba(22, 119, 255, 0.38);
}

.dark .loyalty-tier-level {
  color: #f8fafc;
}

.dark .loyalty-tier-condition {
  color: #94a3b8;
}

.dark .loyalty-tier-tile.is-current .loyalty-tier-level {
  color: #e6f4ff;
}

.dark .loyalty-tier-tile.is-current .loyalty-tier-condition {
  color: #bae0ff;
}

.dark .loyalty-tier-tile.is-current .loyalty-discount-pill {
  background: #1677ff;
  color: white;
  box-shadow: 0 8px 18px rgba(22, 119, 255, 0.22);
}

.dark .loyalty-tier-tile.is-locked .loyalty-tier-level,
.dark .loyalty-tier-tile.is-locked .loyalty-tier-condition {
  color: #64748b;
}

.dark .loyalty-tier-tile.is-locked .loyalty-discount-pill {
  background: rgba(30, 41, 59, 0.72);
  color: #64748b;
  box-shadow: inset 0 0 0 1px rgba(71, 85, 105, 0.78);
}

.dark .loyalty-rules-panel {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.92), rgba(17, 24, 39, 0.82));
}

.dark .loyalty-rule-item {
  border-color: rgba(64, 150, 255, 0.26);
  background: rgba(15, 23, 42, 0.46);
}

.dark .loyalty-rule-item::before {
  background: #69b1ff;
  box-shadow: 0 0 0 0.25rem rgba(22, 119, 255, 0.2);
}

.dark .loyalty-table th {
  background: rgba(30, 41, 59, 0.44);
  color: #94a3b8;
}

.dark .loyalty-table th,
.dark .loyalty-table td {
  border-color: rgba(51, 65, 85, 0.7);
}

.dark .loyalty-table tbody tr:hover {
  background: rgba(0, 44, 140, 0.2);
}

.dark .loyalty-table tbody tr.is-current {
  background: rgba(0, 44, 140, 0.3);
}

.dark .loyalty-table tbody tr.is-current td {
  border-color: rgba(22, 119, 255, 0.34);
  color: #bae0ff;
}

.dark .loyalty-table tbody tr.is-locked td {
  color: #64748b;
}

.dark .loyalty-status-pill.is-current {
  background: rgba(0, 44, 140, 0.44);
  color: #91caff;
  box-shadow: inset 0 0 0 1px rgba(22, 119, 255, 0.44);
}

.dark .loyalty-status-pill.is-unlocked {
  background: rgba(0, 44, 140, 0.38);
  color: #69b1ff;
  box-shadow: inset 0 0 0 1px rgba(64, 150, 255, 0.42);
}

.dark .loyalty-status-pill.is-locked {
  background: rgba(30, 41, 59, 0.68);
  color: #64748b;
  box-shadow: inset 0 0 0 1px rgba(71, 85, 105, 0.78);
}
</style>
