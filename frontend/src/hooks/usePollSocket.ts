import { useCallback, useEffect, useReducer, useRef, useState } from 'react'
import { api, wsUrl } from '../services/api'
import type { ConnStatus, Results } from '../types'
import { applyResults, type ResultsEventKind } from '../utils/results'

type Action = { kind: ResultsEventKind; data: Results }
const reducer = (s: Results | null, a: Action): Results | null => applyResults(s, a.data, a.kind)

export const backoffDelay = (attempt: number, rand: () => number = Math.random) => {
  const cap = Math.min(30_000, 500 * 2 ** attempt)
  return cap / 2 + rand() * (cap / 2) // "equal jitter": avoids thundering herd after an outage
}

/**
 * Live results for one poll over WebSocket.
 * - server sends a snapshot on every (re)connect -> automatic resync
 * - exponential backoff + jitter on reconnect
 * - REST polling fallback while the socket is down; slow resync while live (heals missed pub/sub events)
 */
export function usePollSocket(pollId: string | undefined, initial?: Results) {
  const [results, dispatch] = useReducer(reducer, initial ?? null)
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const statusRef = useRef<ConnStatus>('connecting')
  const set = useCallback((s: ConnStatus) => {
    statusRef.current = s
    setStatus(s)
  }, [])

  const push = useCallback((data: Results) => dispatch({ kind: 'update', data }), [])

  useEffect(() => {
    if (initial) dispatch({ kind: 'snapshot', data: initial })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pollId])

  useEffect(() => {
    if (!pollId) return
    let ws: WebSocket | null = null
    let closed = false
    let attempt = 0
    let retry: ReturnType<typeof setTimeout> | undefined

    const resync = async () => {
      try {
        dispatch({ kind: 'snapshot', data: await api.results(pollId) })
      } catch {
        /* keep last known results */
      }
    }

    const connect = () => {
      if (closed) return
      set(attempt === 0 ? 'connecting' : 'reconnecting')
      ws = new WebSocket(wsUrl(pollId))
      ws.onopen = () => {
        attempt = 0
      }
      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data as string) as { type: string; data: Results }
          if (msg.type === 'snapshot') {
            dispatch({ kind: 'snapshot', data: msg.data })
            set('live')
          } else if (msg.type === 'update') {
            dispatch({ kind: 'update', data: msg.data })
          }
        } catch {
          /* ignore malformed frame */
        }
      }
      ws.onerror = () => ws?.close()
      ws.onclose = () => {
        if (closed) return
        set(navigator.onLine ? 'reconnecting' : 'offline')
        retry = setTimeout(connect, backoffDelay(attempt++))
      }
    }

    const onOnline = () => {
      if (statusRef.current !== 'live') {
        clearTimeout(retry)
        attempt = 0
        ws?.close()
        connect()
      }
    }
    const onOffline = () => set('offline')
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)

    const tick = setInterval(() => {
      if (statusRef.current !== 'live') void resync() // fallback polling while disconnected
    }, 5_000)
    const slow = setInterval(() => {
      if (statusRef.current === 'live') void resync()
    }, 30_000)

    connect()
    return () => {
      closed = true
      clearTimeout(retry)
      clearInterval(tick)
      clearInterval(slow)
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
      ws?.close()
    }
  }, [pollId, set])

  return { results, status, push }
}
