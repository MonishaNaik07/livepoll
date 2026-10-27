import type { PollOption, Results } from '../types'

/**
 * Largest-remainder rounding: whole-number percentages that always sum to exactly 100
 * (naive Math.round can show 33/33/33 = 99 or 34/33/34 = 101). Zero votes -> all 0.
 */
export function percentages(options: PollOption[], r: Pick<Results, 'counts' | 'total'> | null | undefined): Record<string, number> {
  const out: Record<string, number> = {}
  if (!r || r.total <= 0) {
    options.forEach((o) => (out[o.id] = 0))
    return out
  }
  const raw = options.map((o) => ({ id: o.id, exact: ((r.counts[o.id] ?? 0) * 100) / r.total }))
  raw.forEach((x) => (out[x.id] = Math.floor(x.exact)))
  let left = 100 - raw.reduce((s, x) => s + out[x.id], 0)
  raw
    .map((x) => ({ id: x.id, frac: x.exact - Math.floor(x.exact) }))
    .sort((a, b) => b.frac - a.frac)
    .forEach((x) => {
      if (left > 0) {
        out[x.id]++
        left--
      }
    })
  return out
}

export type ResultsEventKind = 'snapshot' | 'update'

/**
 * Snapshots are authoritative (taken from Redis, rebuilt from Mongo if Redis lost data) and always win.
 * Updates are applied only when newer, so duplicated or reordered events can never double-count.
 * `total > cur.total` covers the rare case where Redis was rebuilt and its sequence restarted lower.
 */
export function applyResults(cur: Results | null, incoming: Results, kind: ResultsEventKind): Results {
  if (kind === 'snapshot' || cur === null) return incoming
  if (incoming.pollId !== cur.pollId) return cur
  if (incoming.seq > cur.seq || incoming.total > cur.total) return incoming
  return cur
}

export const fmtNumber = (n: number) => new Intl.NumberFormat().format(n)
export const fmtDate = (iso: string) =>
  new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
