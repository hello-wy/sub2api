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

export function resolveEffectiveRate(options: EffectiveRateOptions): number {
  const userRate = options.userGroupRates?.[options.groupId]
  if (typeof userRate === 'number' && Number.isFinite(userRate)) return userRate
  if (typeof options.groupRate === 'number' && Number.isFinite(options.groupRate)) {
    return options.groupRate
  }
  return DEFAULT_RATE
}

export function applyRateMultiplier(price: NullablePrice, rate: number): number | null {
  const value = toNullableNumber(price)
  if (value === null) return null
  return Number((value * rate).toPrecision(PRICE_PRECISION))
}

export function formatPrice(price: NullablePrice): string {
  const value = toNullableNumber(price)
  if (value === null) return ''
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: DISPLAY_MAX_FRACTION_DIGITS,
  }).format(value)
}
