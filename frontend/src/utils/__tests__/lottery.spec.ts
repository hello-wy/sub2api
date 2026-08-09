import { describe, expect, it, vi } from 'vitest'
import {
  clearPendingLotteryRequestID,
  drawLottery,
  getOrCreatePendingLotteryRequestID,
  getWheelRotationForPrize,
  lotteryPrizeFromSnapshot,
  pickLotteryPrize,
  readPendingLotteryRequestID,
} from '../lottery'

describe('lottery', () => {
	function createStorage() {
		const values = new Map<string, string>()
		return {
			getItem: (key: string) => values.get(key) ?? null,
			setItem: (key: string, value: string) => values.set(key, value),
			removeItem: (key: string) => values.delete(key),
		}
	}

	it('同一待处理操作复用请求 ID，成功后才释放', () => {
		const storage = createStorage()
		const createRequestID = vi.fn(() => 'lottery-request-123')

		const first = getOrCreatePendingLotteryRequestID(storage, 'pending', createRequestID)
		const retry = getOrCreatePendingLotteryRequestID(storage, 'pending', createRequestID)

		expect(first).toBe('lottery-request-123')
		expect(retry).toBe(first)
		expect(createRequestID).toHaveBeenCalledTimes(1)
		clearPendingLotteryRequestID(storage, 'pending', first)
		expect(readPendingLotteryRequestID(storage, 'pending')).toBeUndefined()
	})

	it('不会由过期响应清除更新后的待处理请求 ID', () => {
		const storage = createStorage()
		storage.setItem('pending', 'lottery-request-new')

		clearPendingLotteryRequestID(storage, 'pending', 'lottery-request-old')

		expect(readPendingLotteryRequestID(storage, 'pending')).toBe('lottery-request-new')
	})

	it('历史奖项始终使用开奖快照', () => {
		const prize = lotteryPrizeFromSnapshot({
			prize_id: 'quota-10',
			prize_label: '$10',
			prize_type: 'balance',
			amount: 10,
		})

		expect(prize).toMatchObject({ id: 'quota-10', label: '$10', kind: 'quota', amount: 10 })
	})

  it('按奖池概率区间选择奖项', () => {
    const pool = [
      { id: 'a', label: 'A', detail: '', probability: 50, kind: 'none' as const },
      { id: 'b', label: 'B', detail: '', probability: 50, kind: 'quota' as const },
    ]
    expect(pickLotteryPrize(pool, () => 0.49).id).toBe('a')
    expect(pickLotteryPrize(pool, () => 0.5).id).toBe('b')
  })

  it('连续四次未中奖后必定进入保底池并重置进度', () => {
    const result = drawLottery(4, () => 0)
    expect(result.isGuaranteed).toBe(true)
    expect(result.prize.id).toBe('quota-10')
    expect(result.nextMisses).toBe(0)
  })

  it('未中奖时最多累计到四次', () => {
    expect(drawLottery(3, () => 0).nextMisses).toBe(4)
  })

  it('连续抽奖时，目标扇区始终停在指针下方', () => {
    const firstRotation = getWheelRotationForPrize(0, 4, 7)
    const secondRotation = getWheelRotationForPrize(firstRotation, 1, 7)
    const sliceAngle = 360 / 7

    expect(firstRotation % 360).toBeCloseTo((360 - 4 * sliceAngle) % 360)
    expect(secondRotation % 360).toBeCloseTo((360 - sliceAngle) % 360)
  })
})
