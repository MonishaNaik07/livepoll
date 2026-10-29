import type { PollOption, Results } from '../types'
import { fmtNumber, percentages } from '../utils/results'

interface Props { options: PollOption[]; results: Results | null; highlightId?: string }

export function ResultBars({ options, results, highlightId }: Props) {
  const pct = percentages(options, results)
  const total = results?.total ?? 0
  const max = Math.max(0, ...options.map((o) => results?.counts[o.id] ?? 0))
  return (
    <div className="results" aria-label="Poll results">
      <ul className="bars">
        {options.map((o) => {
          const n = results?.counts[o.id] ?? 0
          const leading = total > 0 && n === max
          return (
            <li key={o.id} className={`bar-row${leading ? ' leading' : ''}`}>
              <div className="bar-head">
                <span className="bar-label">
                  {o.text}
                  {leading && <span className="chip lead">Leading</span>}
                  {highlightId === o.id && <span className="chip">Your vote</span>}
                </span>
                <span className="bar-num"><strong>{pct[o.id]}%</strong> · {fmtNumber(n)} {n === 1 ? 'vote' : 'votes'}</span>
              </div>
              <div className="bar-track" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct[o.id]} aria-label={`${o.text}: ${pct[o.id]} percent`}>
                <div className="bar-fill" style={{ width: `${pct[o.id]}%` }} />
              </div>
            </li>
          )
        })}
      </ul>
      <p className="total" aria-live="polite">
        {total === 0 ? 'No votes yet. Be the first!' : `${fmtNumber(total)} ${total === 1 ? 'vote' : 'votes'} total`}
      </p>
    </div>
  )
}