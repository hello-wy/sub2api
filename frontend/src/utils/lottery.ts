export type LotteryPrizeKind = 'none' | 'quota' | 'voucher'

export interface LotteryPrize {
  id: string
  label: string
  detail: string
  probability: number
  kind: LotteryPrizeKind
  amount?: number
}

export interface LotteryPrizeSnapshot {
  prize_id: string
  prize_label: string
  prize_type: 'none' | 'balance' | 'subscription'
  amount: number
}

export function lotteryPrizeFromSnapshot(draw: LotteryPrizeSnapshot): LotteryPrize {
  return {
    id: draw.prize_id,
    label: draw.prize_label,
    detail: draw.prize_type === 'none' ? '下次好运会来' : '奖励已发放到账户',
    probability: 0,
    kind: draw.prize_type === 'balance' ? 'quota' : draw.prize_type === 'subscription' ? 'voucher' : 'none',
    amount: draw.amount,
  }
}

export const lotteryPrizePool: LotteryPrize[] = [
  { id: 'none', label: '谢谢参与', detail: '下次好运会来', probability: 50, kind: 'none' },
  { id: 'quota-10', label: '$10', detail: '幸运奖励已发放', probability: 31, kind: 'quota', amount: 10 },
  { id: 'quota-30', label: '$30', detail: '幸运奖励已发放', probability: 11, kind: 'quota', amount: 30 },
  { id: 'quota-100', label: '$100', detail: '幸运奖励已发放', probability: 5, kind: 'quota', amount: 100 },
  { id: 'quota-1000', label: '$1000', detail: '幸运奖励已发放', probability: 0.1, kind: 'quota', amount: 1000 },
]

export const guaranteedPrizePool: LotteryPrize[] = [
  { id: 'quota-10', label: '$10', detail: '保底奖励已发放', probability: 70, kind: 'quota', amount: 10 },
  { id: 'quota-30', label: '$30', detail: '保底奖励已发放', probability: 20, kind: 'quota', amount: 30 },
  { id: 'quota-100', label: '$100', detail: '保底奖励已发放', probability: 7, kind: 'quota', amount: 100 },
]

export interface LotteryDrawResult {
  prize: LotteryPrize
  isGuaranteed: boolean
  nextMisses: number
}

export type LotteryRequestStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function createLotteryRequestID(): string {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  return `lottery-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function readPendingLotteryRequestID(storage: LotteryRequestStorage | undefined, storageKey: string): string | undefined {
  if (!storage) return undefined
  try {
    const value = storage.getItem(storageKey)?.trim()
    if (value && value.length >= 8 && value.length <= 128) return value
    if (value) storage.removeItem(storageKey)
  } catch {
    // In-memory state still protects duplicate clicks when browser storage is unavailable.
  }
  return undefined
}

export function getOrCreatePendingLotteryRequestID(
  storage: LotteryRequestStorage | undefined,
  storageKey: string,
  createRequestID = createLotteryRequestID,
): string {
  const existing = readPendingLotteryRequestID(storage, storageKey)
  if (existing) return existing
  const requestID = createRequestID()
  try {
    storage?.setItem(storageKey, requestID)
  } catch {
    // The caller retains the returned ID in memory.
  }
  return requestID
}

export function clearPendingLotteryRequestID(
  storage: LotteryRequestStorage | undefined,
  storageKey: string,
  expectedRequestID?: string,
): void {
  if (!storage) return
  try {
    if (expectedRequestID && storage.getItem(storageKey) !== expectedRequestID) return
    storage.removeItem(storageKey)
  } catch {
    // Clearing browser storage is best-effort; the caller also clears memory state.
  }
}

export function pickLotteryPrize(pool: LotteryPrize[], random = Math.random): LotteryPrize {
  const total = pool.reduce((sum, prize) => sum + prize.probability, 0)
  if (total <= 0) throw new Error('抽奖奖池概率必须大于 0')

  let cursor = Math.min(Math.max(random(), 0), 0.999999999) * total
  for (const prize of pool) {
    cursor -= prize.probability
    if (cursor < 0) return prize
  }
  return pool[pool.length - 1]
}

export function drawLottery(misses: number, random = Math.random): LotteryDrawResult {
  const isGuaranteed = misses >= 4
  const prize = pickLotteryPrize(isGuaranteed ? guaranteedPrizePool : lotteryPrizePool, random)
  return {
    prize,
    isGuaranteed,
    nextMisses: prize.kind === 'none' ? Math.min(misses + 1, 4) : 0,
  }
}

export function getWheelRotationForPrize(currentRotation: number, targetIndex: number, prizeCount: number, fullTurns = 4): number {
  if (!Number.isInteger(targetIndex) || targetIndex < 0 || targetIndex >= prizeCount) throw new Error('目标奖品索引无效')
  if (!Number.isInteger(prizeCount) || prizeCount <= 0) throw new Error('奖品数量必须大于 0')

  const sliceAngle = 360 / prizeCount
  const currentOffset = ((currentRotation % 360) + 360) % 360
  const targetOffset = (360 - targetIndex * sliceAngle) % 360
  const correction = (targetOffset - currentOffset + 360) % 360
  return currentRotation + fullTurns * 360 + correction
}
