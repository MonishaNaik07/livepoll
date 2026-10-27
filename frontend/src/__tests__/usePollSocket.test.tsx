import { act, renderHook } from '@testing-library/react'
import { backoffDelay, usePollSocket } from '../hooks/usePollSocket'
import type { Results } from '../types'

vi.mock('../services/api', () => ({
  wsUrl: (id: string) => `ws://test/api/ws/polls/${id}`,
  api: { results: vi.fn().mockRejectedValue(new Error('offline')) },
}))

class FakeWS {
  static instances: FakeWS[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  constructor(public url: string) { FakeWS.instances.push(this) }
  close() { this.closed = true; this.onclose?.() }
  emit(type: string, data: Results) { this.onmessage?.({ data: JSON.stringify({ type, data }) }) }
}

const res = (seq: number, o1: number, o2 = 0): Results => ({ pollId: 'p1', status: 'open', seq, total: o1 + o2, counts: { o1, o2 } })

beforeEach(() => { FakeWS.instances = []; vi.stubGlobal('WebSocket', FakeWS); vi.useFakeTimers() })
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

describe('usePollSocket', () => {
  it('shows a snapshot, goes live, then applies live updates without a refresh', () => {
    const { result } = renderHook(() => usePollSocket('p1'))
    expect(result.current.status).toBe('connecting')
    act(() => FakeWS.instances[0].emit('snapshot', res(1, 1)))
    expect(result.current.status).toBe('live')
    expect(result.current.results?.total).toBe(1)
    act(() => FakeWS.instances[0].emit('update', res(2, 1, 1)))
    expect(result.current.results?.total).toBe(2)
  })

  it('ignores duplicate events', () => {
    const { result } = renderHook(() => usePollSocket('p1'))
    act(() => FakeWS.instances[0].emit('snapshot', res(1, 1)))
    act(() => FakeWS.instances[0].emit('update', res(2, 2)))
    act(() => FakeWS.instances[0].emit('update', res(2, 2)))
    act(() => FakeWS.instances[0].emit('update', res(1, 1)))
    expect(result.current.results?.total).toBe(2)
  })

  it('reconnects with backoff after a drop and resynchronises from the new snapshot', () => {
    const { result } = renderHook(() => usePollSocket('p1'))
    act(() => FakeWS.instances[0].emit('snapshot', res(1, 1)))
    act(() => FakeWS.instances[0].close()) // connection lost
    expect(result.current.status).toBe('reconnecting')
    expect(FakeWS.instances).toHaveLength(1)
    act(() => { vi.advanceTimersByTime(600) }) // first backoff <= 500ms
    expect(FakeWS.instances).toHaveLength(2)
    // votes happened while we were away; the server snapshot catches us up
    act(() => FakeWS.instances[1].emit('snapshot', res(9, 4, 3)))
    expect(result.current.status).toBe('live')
    expect(result.current.results?.total).toBe(7)
  })

  it('cleans up the socket on unmount', () => {
    const { unmount } = renderHook(() => usePollSocket('p1'))
    unmount()
    expect(FakeWS.instances[0].closed).toBe(true)
  })
})

describe('backoffDelay', () => {
  it('grows exponentially, stays within jitter bounds and caps at 30s', () => {
    expect(backoffDelay(0, () => 0)).toBe(250)
    expect(backoffDelay(0, () => 1)).toBe(500)
    expect(backoffDelay(3, () => 1)).toBe(4000)
    expect(backoffDelay(20, () => 1)).toBe(30000)
  })
})
