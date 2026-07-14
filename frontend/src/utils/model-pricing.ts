const MTOK = 1_000_000
const DEFAULT_RATE = 1
const PRICE_PRECISION = 10
const DISPLAY_MAX_FRACTION_DIGITS = 6

export type NullablePrice = number | string | null | undefined

export interface EffectiveRateOptions {
  readonly groupId: number
  readonly groupRate: number | null | undefined
  readonly userGroupRates?: Readonly<Record<number, number>>
}

export interface PricingGroup {
  readonly id: number
  readonly name?: string
  readonly rate_multiplier: number | null | undefined
}

export interface CardGroupRateOptions {
  readonly groups: readonly PricingGroup[]
  readonly selectedGroupId?: number | string | null
  readonly userGroupRates?: Readonly<Record<number, number>>
}

export function toNullableNumber(value: NullablePrice): number | null {
  if (value === null || value === undefined || value === '') return null
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : null
}

export function toDisplayTokenPrice(perTokenPrice: NullablePrice): number | null {
  const price = toNullableNumber(perTokenPrice)
  if (price === null) return null
  return Number((price * MTOK).toPrecision(PRICE_PRECISION))
}

function positiveRateOrDefault(rate: unknown): number {
  return typeof rate === 'number' && Number.isFinite(rate) && rate > 0 ? rate : DEFAULT_RATE
}

export function resolveEffectiveRate(options: EffectiveRateOptions): number {
  const userRate = options.userGroupRates?.[options.groupId]
  if (typeof userRate === 'number' && Number.isFinite(userRate) && userRate > 0) return userRate
  return positiveRateOrDefault(options.groupRate)
}

export function resolveCardGroup<T extends PricingGroup>(options: Omit<CardGroupRateOptions, 'groups'> & { readonly groups: readonly T[] }): T | undefined {
  const selectedGroup = options.groups.find((group) => String(group.id) === String(options.selectedGroupId))
  if (selectedGroup) return selectedGroup

  return [...options.groups].sort((left, right) => {
    const rateDifference = resolveEffectiveRate({
      groupId: left.id,
      groupRate: left.rate_multiplier,
      userGroupRates: options.userGroupRates,
    }) - resolveEffectiveRate({
      groupId: right.id,
      groupRate: right.rate_multiplier,
      userGroupRates: options.userGroupRates,
    })
    if (rateDifference !== 0) return rateDifference
    const nameDifference = (left.name ?? '').localeCompare(right.name ?? '')
    return nameDifference !== 0 ? nameDifference : left.id - right.id
  })[0]
}

export function resolveCardGroupRate(options: CardGroupRateOptions): number {
  const group = resolveCardGroup(options)
  if (!group) return DEFAULT_RATE
  return resolveEffectiveRate({
    groupId: group.id,
    groupRate: group.rate_multiplier,
    userGroupRates: options.userGroupRates,
  })
}

export function normalizeRechargeMultiplier(multiplier: NullablePrice): number {
  const value = toNullableNumber(multiplier)
  return value !== null && value > 0 ? value : DEFAULT_RATE
}

export function discountRatio(groupRate: number, rechargeMultiplier: number): number {
  return positiveRateOrDefault(groupRate) / normalizeRechargeMultiplier(rechargeMultiplier)
}

export function formatDiscount(groupRate: number, rechargeMultiplier: number): string {
  const ratio = discountRatio(groupRate, rechargeMultiplier)
  if (ratio >= 1) return ''
  return `${formatPrice(Number((ratio * 10).toPrecision(PRICE_PRECISION)))}折`
}

export function applyRateMultiplier(price: NullablePrice, rate: number): number | null {
  const value = toNullableNumber(price)
  if (value === null) return null
  return Number((value * rate).toPrecision(PRICE_PRECISION))
}

export function basePrice(perTokenPrice: NullablePrice): number | null {
  return toDisplayTokenPrice(perTokenPrice)
}

export function quotaPrice(perTokenPrice: NullablePrice, groupRate: number): number | null {
  return applyRateMultiplier(basePrice(perTokenPrice), positiveRateOrDefault(groupRate))
}

export function equivalentPrice(
  perTokenPrice: NullablePrice,
  groupRate: number,
  rechargeMultiplier: number,
): number | null {
  return applyRateMultiplier(basePrice(perTokenPrice), discountRatio(groupRate, rechargeMultiplier))
}

export function formatPrice(price: NullablePrice): string {
  const value = toNullableNumber(price)
  if (value === null) return ''
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: DISPLAY_MAX_FRACTION_DIGITS,
  }).format(value)
}
