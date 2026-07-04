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
          <div v-if="!pointsDefinition" class="loyalty-panel border-amber-200 bg-amber-50/90 p-5 dark:border-amber-900/50 dark:bg-amber-900/20">
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
                  <div class="text-center">
                    <p class="text-base font-black text-slate-950 dark:text-white">{{ tier.rule.level }}</p>
                    <span class="loyalty-tier-emoji">{{ tier.icon }}</span>
                    <p class="mt-3 text-sm font-semibold text-slate-500 dark:text-dark-400">
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

          <section class="loyalty-panel p-5 sm:p-6">
            <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
              <div
                v-for="item in ruleNotes"
                :key="item.title"
                class="loyalty-note-item"
              >
                <div class="loyalty-note-icon">
                  <Icon :name="item.icon" size="md" />
                </div>
                <div class="min-w-0">
                  <h4 class="text-base font-black tracking-normal text-slate-950 dark:text-white">{{ item.title }}</h4>
                  <p class="mt-1 text-sm font-medium leading-6 text-slate-500 dark:text-dark-400">{{ item.description }}</p>
                </div>
              </div>
            </div>
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
                    <th>{{ t('loyalty.tableDescription') }}</th>
                    <th class="text-right">{{ t('loyalty.tableStatus') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in benefitRows" :key="`${row.scope}-${row.level}`">
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
                    <td class="text-base font-black text-emerald-600 dark:text-emerald-300">{{ row.discount }}</td>
                    <td class="min-w-[280px] text-slate-500 dark:text-dark-400">{{ row.description }}</td>
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
import { userAPI } from '@/api'
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
  icon: string
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
  description: string
  state: RuleState
}

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(true)
const definitions = ref<UserAttributeDefinition[]>([])
const values = ref<UserAttributeValue[]>([])

const weeklyRules = weeklyLoyaltyRules
const permanentRules = permanentLoyaltyRules

const pointDefinitions = computed(() => findLoyaltyPointsDefinitions(definitions.value))
const pointsDefinition = computed(() => Boolean(pointDefinitions.value.weekly && pointDefinitions.value.permanent))
const weeklyPoints = computed(() => readLoyaltyPoints(definitions.value, values.value, 'weekly'))
const permanentPoints = computed(() => readLoyaltyPoints(definitions.value, values.value, 'permanent'))
const weeklyProgress = computed(() => resolveLoyaltyProgress(weeklyPoints.value, weeklyRules))
const permanentProgress = computed(() => resolveLoyaltyProgress(permanentPoints.value, permanentRules))
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
    tiers: buildTierCards(weeklyRules, weeklyProgress.value, weeklyPoints.value),
  },
  {
    key: 'permanent',
    title: t('loyalty.permanentPlan'),
    hint: t('loyalty.permanentHint'),
    badge: t('loyalty.permanentBadge'),
    icon: 'link',
    tiers: buildTierCards(permanentRules, permanentProgress.value, permanentPoints.value),
  },
])

const ruleNotes = computed(() => [
  {
    title: t('loyalty.ruleHigherTitle'),
    description: t('loyalty.ruleHigherDesc'),
    icon: 'trendingUp' as IconName,
  },
  {
    title: t('loyalty.weeklyResetTitle'),
    description: t('loyalty.weeklyResetDesc'),
    icon: 'calendar' as IconName,
  },
  {
    title: t('loyalty.permanentStableTitle'),
    description: t('loyalty.permanentStableDesc'),
    icon: 'shield' as IconName,
  },
  {
    title: t('loyalty.earnTitle'),
    description: t('loyalty.earnRechargeDesc'),
    icon: 'gift' as IconName,
  },
  {
    title: t('loyalty.bonusTitle'),
    description: t('loyalty.bonusDesc'),
    icon: 'users' as IconName,
  },
])

const benefitRows = computed<BenefitRow[]>(() => [
  ...buildBenefitRows('weekly', weeklyRules, weeklyProgress.value, weeklyPoints.value),
  ...buildBenefitRows('permanent', permanentRules, permanentProgress.value, permanentPoints.value),
])

function buildTierCards(rules: LoyaltyRule[], progress: LoyaltyProgress, points: number): TierCard[] {
  const iconsByLevel: Record<string, string> = {
    L1: '🌱',
    L2: '⭐',
    L3: '👑',
    L4: '🏆',
  }
  return rules.map((rule) => ({
    rule,
    state: ruleState(rule, progress, points),
    icon: iconsByLevel[rule.level] ?? '⭐',
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
    description: scope === 'weekly' ? t('loyalty.weeklyRuleDescription') : t('loyalty.permanentRuleDescription'),
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
    const resp = await userAPI.getMyAttributes()
    definitions.value = resp.definitions
    values.value = resp.values
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

function goRecharge(): void {
  void router.push('/purchase')
}

onMounted(() => {
  void loadAttributes()
})
</script>

<style scoped>
.loyalty-shell {
  position: relative;
  background:
    linear-gradient(120deg, rgba(232, 255, 247, 0.72) 0%, rgba(248, 251, 252, 0.97) 44%, rgba(240, 249, 255, 0.9) 100%);
}

.loyalty-panel {
  border: 1px solid rgba(213, 231, 227, 0.92);
  border-radius: 0.5rem;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.055);
}

.loyalty-hero {
  min-height: 8rem;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.065);
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
  border: 1px solid rgba(167, 243, 208, 0.78);
  background: rgba(236, 253, 245, 0.84);
  box-shadow: 0 12px 26px rgba(16, 185, 129, 0.12);
}

.loyalty-mascot-inner {
  display: flex;
  height: 3.75rem;
  width: 3.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  background: linear-gradient(135deg, #34d399, #10b981);
  color: white;
  box-shadow: 0 10px 20px rgba(16, 185, 129, 0.22);
}

.loyalty-recharge-btn {
  display: inline-flex;
  min-height: 3.5rem;
  align-items: center;
  justify-content: center;
  gap: 0.625rem;
  border-radius: 0.5rem;
  background: linear-gradient(135deg, #10b981 0%, #00c786 58%, #05b981 100%);
  padding: 0 1.5rem;
  font-size: 1rem;
  font-weight: 900;
  color: white;
  box-shadow: 0 12px 26px rgba(16, 185, 129, 0.24);
  transition: transform 160ms ease, box-shadow 160ms ease;
}

.loyalty-recharge-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 16px 32px rgba(16, 185, 129, 0.28);
}

.loyalty-recharge-btn:focus-visible {
  outline: 3px solid rgba(16, 185, 129, 0.26);
  outline-offset: 3px;
}

.loyalty-alert-icon,
.loyalty-section-icon,
.loyalty-note-icon,
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
.loyalty-note-icon,
.loyalty-table-plan-icon {
  height: 2.75rem;
  width: 2.75rem;
  background: rgba(209, 250, 229, 0.78);
  color: #059669;
  box-shadow: inset 0 0 0 1px rgba(167, 243, 208, 0.95);
}

.loyalty-stat-card {
  min-height: 7.75rem;
  border: 1px solid rgba(213, 231, 227, 0.92);
  border-radius: 0.5rem;
  background: rgba(255, 255, 255, 0.94);
  padding: 1.25rem;
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.055);
}

.loyalty-stat-icon {
  height: 3.25rem;
  width: 3.25rem;
  background: linear-gradient(135deg, rgba(209, 250, 229, 0.88), rgba(204, 251, 241, 0.82));
  color: #059669;
  box-shadow: inset 0 0 0 1px rgba(167, 243, 208, 0.82);
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
  min-height: 8rem;
  flex-direction: column;
  align-items: center;
  justify-content: space-between;
  border-radius: 0.5rem;
  border: 1px solid rgba(204, 251, 241, 0.98);
  background: rgba(255, 255, 255, 0.7);
  padding: 1rem 0.75rem;
  text-align: center;
}

.loyalty-tier-tile.is-current {
  border-color: rgba(52, 211, 153, 0.86);
  background: rgba(236, 253, 245, 0.96);
  box-shadow: 0 14px 30px rgba(16, 185, 129, 0.12);
}

.loyalty-tier-tile.is-unlocked {
  border-color: rgba(153, 246, 228, 0.98);
  background: rgba(240, 253, 250, 0.86);
}

.loyalty-tier-tile.is-locked {
  border-color: rgba(226, 232, 240, 0.96);
  background: rgba(255, 255, 255, 0.72);
}

.loyalty-tier-emoji {
  display: flex;
  height: 2.2rem;
  align-items: center;
  justify-content: center;
  margin-top: 0.45rem;
  font-size: 1.55rem;
  line-height: 1;
}

.loyalty-discount-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: rgba(209, 250, 229, 0.82);
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 900;
  color: #059669;
  box-shadow: inset 0 0 0 1px rgba(167, 243, 208, 0.95);
}

.loyalty-note-item {
  display: flex;
  min-height: 7.5rem;
  gap: 1rem;
  border: 1px solid rgba(204, 251, 241, 0.86);
  border-radius: 0.5rem;
  background: rgba(255, 255, 255, 0.62);
  padding: 1rem;
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
  background: rgba(236, 253, 245, 0.9);
  color: #059669;
}

.loyalty-table {
  width: 100%;
  min-width: 900px;
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
  background: rgba(240, 253, 250, 0.42);
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
  background: rgba(209, 250, 229, 0.9);
  color: #059669;
  box-shadow: inset 0 0 0 1px rgba(110, 231, 183, 0.95);
}

.loyalty-status-pill.is-unlocked {
  background: rgba(204, 251, 241, 0.86);
  color: #0f766e;
  box-shadow: inset 0 0 0 1px rgba(94, 234, 212, 0.9);
}

.loyalty-status-pill.is-locked {
  background: rgba(254, 243, 199, 0.9);
  color: #d97706;
  box-shadow: inset 0 0 0 1px rgba(252, 211, 77, 0.86);
}

:global(.dark) .loyalty-shell {
  background:
    linear-gradient(120deg, rgba(2, 44, 34, 0.98) 0%, rgba(2, 6, 23, 0.98) 44%, rgba(8, 47, 73, 0.96) 100%);
}

:global(.dark) .loyalty-panel,
:global(.dark) .loyalty-stat-card {
  border-color: rgba(51, 65, 85, 0.82);
  background: rgba(15, 23, 42, 0.86);
  box-shadow: 0 14px 34px rgba(0, 0, 0, 0.22);
}

:global(.dark) .loyalty-hero {
  background: rgba(15, 23, 42, 0.86);
}

:global(.dark) .loyalty-mascot {
  border-color: rgba(6, 95, 70, 0.78);
  background: rgba(6, 78, 59, 0.5);
}

:global(.dark) .loyalty-plan-badge,
:global(.dark) .loyalty-table-action {
  background: rgba(15, 23, 42, 0.76);
  color: #cbd5e1;
  box-shadow: inset 0 0 0 1px rgba(51, 65, 85, 0.9);
}

:global(.dark) .loyalty-section-icon,
:global(.dark) .loyalty-note-icon,
:global(.dark) .loyalty-table-plan-icon,
:global(.dark) .loyalty-stat-icon {
  background: rgba(6, 78, 59, 0.45);
  color: #6ee7b7;
  box-shadow: inset 0 0 0 1px rgba(6, 95, 70, 0.72);
}

:global(.dark) .loyalty-tier-tile.is-current {
  border-color: rgba(16, 185, 129, 0.76);
  background: rgba(6, 78, 59, 0.28);
}

:global(.dark) .loyalty-tier-tile.is-unlocked {
  border-color: rgba(20, 184, 166, 0.52);
  background: rgba(19, 78, 74, 0.24);
}

:global(.dark) .loyalty-tier-tile.is-locked {
  border-color: rgba(51, 65, 85, 0.92);
  background: rgba(15, 23, 42, 0.62);
}

:global(.dark) .loyalty-discount-pill {
  background: rgba(6, 78, 59, 0.45);
  color: #6ee7b7;
  box-shadow: inset 0 0 0 1px rgba(6, 95, 70, 0.72);
}

:global(.dark) .loyalty-table th {
  background: rgba(15, 23, 42, 0.58);
}

:global(.dark) .loyalty-table th,
:global(.dark) .loyalty-table td {
  border-color: rgba(51, 65, 85, 0.76);
}

:global(.dark) .loyalty-table tbody tr:hover {
  background: rgba(6, 78, 59, 0.18);
}

:global(.dark) .loyalty-note-item {
  border-color: rgba(20, 184, 166, 0.3);
  background: rgba(15, 23, 42, 0.42);
}

:global(.dark) .loyalty-status-pill.is-current {
  background: rgba(6, 78, 59, 0.42);
  color: #6ee7b7;
  box-shadow: inset 0 0 0 1px rgba(6, 95, 70, 0.86);
}

:global(.dark) .loyalty-status-pill.is-unlocked {
  background: rgba(19, 78, 74, 0.42);
  color: #5eead4;
  box-shadow: inset 0 0 0 1px rgba(20, 184, 166, 0.72);
}

:global(.dark) .loyalty-status-pill.is-locked {
  background: rgba(120, 53, 15, 0.36);
  color: #fbbf24;
  box-shadow: inset 0 0 0 1px rgba(180, 83, 9, 0.72);
}
</style>
