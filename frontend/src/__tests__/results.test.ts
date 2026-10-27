import { applyResults, percentages } from '../utils/results'
import type { Results } from '../types'

const opts = [{ id: 'o1', text: 'A' }, { id: 'o2', text: 'B' }, { id: 'o3', text: 'C' }]
const r = (seq: number, c: [number, number, number], status: 'open' | 'closed' = 'open'): Results => ({
  pollId: 'p', status, seq, total: c[0] + c[1] + c[2], counts: { o1: c[0], o2: c[1], o3: c[2] },
})

describe('percentages', () => {
  it('is all zeros with no votes', () => {
    expect(percentages(opts, r(0, [0, 0, 0]))).toEqual({ o1: 0, o2: 0, o3: 0 })
  })
  it('always sums to exactly 100 (largest remainder)', () => {
    for (const c of [[1, 1, 1], [1, 2, 4], [3, 3, 1], [7, 0, 0], [5, 5, 5]] as [number, number, number][]) {
      const p = percentages(opts, r(1, c))
      expect(Object.values(p).reduce((a, b) => a + b, 0)).toBe(100)
    }
  })
  it('splits 1/1/1 as 34/33/33', () => {
    expect(Object.values(percentages(opts, r(1, [1, 1, 1]))).sort()).toEqual([33, 33, 34])
  })
})

describe('applyResults', () => {
  it('applies a newer update', () => expect(applyResults(r(1, [1, 0, 0]), r(2, [1, 1, 0]), 'update').total).toBe(2))
  it('ignores a duplicate update (same seq) so counts are never double applied', () => {
    const cur = r(2, [1, 1, 0])
    expect(applyResults(cur, r(2, [1, 1, 0]), 'update')).toBe(cur)
  })
  it('ignores a stale, out-of-order update', () => {
    const cur = r(5, [3, 1, 1])
    expect(applyResults(cur, r(3, [2, 1, 0]), 'update')).toBe(cur)
  })
  it('snapshot is authoritative even if seq is lower (Redis rebuilt)', () => {
    expect(applyResults(r(50, [9, 0, 0]), r(1, [9, 0, 0]), 'snapshot').seq).toBe(1)
  })
  it('applies status change carried by a newer seq', () => {
    expect(applyResults(r(2, [1, 0, 0]), r(3, [1, 0, 0], 'closed'), 'update').status).toBe('closed')
  })
})
