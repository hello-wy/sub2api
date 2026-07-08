import type { UserAttributeDefinition, UserAttributeValue } from '@/types'

export const LOYALTY_WEEKLY_POINTS_ATTRIBUTE_KEY = 'loyalty_weekly_points'
export const LOYALTY_PERMANENT_POINTS_ATTRIBUTE_KEY = 'loyalty_permanent_points'

export type LoyaltyScope = 'weekly' | 'permanent'

export interface LoyaltyRule {
  scope: LoyaltyScope
  level: string
  points: number
  discount: number
}

export interface LoyaltyProgress {
  current: LoyaltyRule | null
  next: LoyaltyRule | null
  progressPercent: number
  remainingPoints: number
}

export const weeklyLoyaltyRules: LoyaltyRule[] = [
  { scope: 'weekly', level: 'L1', points: 20, discount: 2 },
  { scope: 'weekly', level: 'L2', points: 200, discount: 4 },
  { scope: 'weekly', level: 'L3', points: 400, discount: 6 },
  { scope: 'weekly', level: 'L4', points: 800, discount: 8 },
]

export const permanentLoyaltyRules: LoyaltyRule[] = [
  { scope: 'permanent', level: 'L2', points: 800, discount: 4 },
  { scope: 'permanent', level: 'L3', points: 4000, discount: 6 },
  { scope: 'permanent', level: 'L4', points: 8000, discount: 8 },
]

export function loyaltyAttributeKeyForScope(scope: LoyaltyScope): string {
  return scope === 'weekly'
    ? LOYALTY_WEEKLY_POINTS_ATTRIBUTE_KEY
    : LOYALTY_PERMANENT_POINTS_ATTRIBUTE_KEY
}

export function findLoyaltyPointsDefinition(
  definitions: UserAttributeDefinition[],
  scope: LoyaltyScope,
): UserAttributeDefinition | null {
  const key = loyaltyAttributeKeyForScope(scope)
  return definitions.find((def) => (
    def.key === key
    && def.type === 'number'
    && def.enabled !== false
  )) ?? null
}

export function findLoyaltyPointsDefinitions(
  definitions: UserAttributeDefinition[],
): Record<LoyaltyScope, UserAttributeDefinition | null> {
  return {
    weekly: findLoyaltyPointsDefinition(definitions, 'weekly'),
    permanent: findLoyaltyPointsDefinition(definitions, 'permanent'),
  }
}

export function readLoyaltyPoints(
  definitions: UserAttributeDefinition[],
  values: UserAttributeValue[],
  scope: LoyaltyScope,
  now: Date = new Date(),
): number {
  const definition = findLoyaltyPointsDefinition(definitions, scope)
  if (!definition) return 0

  const matched = values.find((value) => value.attribute_id === definition.id)
  if (!matched || (scope === 'weekly' && !isAttributeValueInCurrentWeek(matched, now))) {
    return 0
  }

  const parsed = Number(matched.value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

function isAttributeValueInCurrentWeek(value: UserAttributeValue, now: Date): boolean {
  if (!value.updated_at) return false
  const updatedAt = new Date(value.updated_at)
  if (Number.isNaN(updatedAt.getTime())) return false
  return updatedAt >= startOfCurrentWeek(now)
}

function startOfCurrentWeek(now: Date): Date {
  const start = new Date(now)
  const day = start.getDay() || 7
  start.setDate(start.getDate() - day + 1)
  start.setHours(0, 0, 0, 0)
  return start
}

export function resolveLoyaltyProgress(points: number, rules: LoyaltyRule[]): LoyaltyProgress {
  const normalizedPoints = Number.isFinite(points) && points > 0 ? points : 0
  const sortedRules = [...rules].sort((a, b) => a.points - b.points)
  const current = [...sortedRules].reverse().find((rule) => normalizedPoints >= rule.points) ?? null
  const next = sortedRules.find((rule) => normalizedPoints < rule.points) ?? null

  if (!next) {
    return {
      current,
      next: null,
      progressPercent: 100,
      remainingPoints: 0,
    }
  }

  const previousPoints = current?.points ?? 0
  const range = Math.max(1, next.points - previousPoints)
  const progressPercent = Math.min(
    100,
    Math.max(0, ((normalizedPoints - previousPoints) / range) * 100),
  )

  return {
    current,
    next,
    progressPercent,
    remainingPoints: Math.max(0, next.points - normalizedPoints),
  }
}

export function formatLoyaltyPoints(points: number): string {
  const normalized = Number.isFinite(points) && points > 0 ? points : 0
  return Number.isInteger(normalized) ? String(normalized) : normalized.toFixed(2)
}
