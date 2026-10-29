import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { EyeIcon, ShareIcon, TrashIcon } from '../components/Icons'
import { Skeleton } from '../components/Skeleton'
import { useAuth } from '../contexts/AuthContext'
import { useToast } from '../contexts/ToastContext'
import { api, ApiError } from '../services/api'
import type { Poll, PollPage } from '../types'
import { copyText } from '../utils/clipboard'
import { fmtDate, fmtNumber, percentages } from '../utils/results'

export default function Dashboard() {
  const toast = useToast()
  const { user } = useAuth()
  const [page, setPage] = useState(1)
  const [data, setData] = useState<PollPage | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [toDelete, setToDelete] = useState<Poll | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setData(await api.listPolls(page))
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Could not load polls')
    } finally {
      setLoading(false)
    }
  }, [page])
  useEffect(() => { void load() }, [load])

  const confirmDelete = async () => {
    if (!toDelete) return
    setBusy(true)
    try {
      await api.remove(toDelete.id)
      toast('Poll deleted', 'success')
      setToDelete(null)
      if (data && data.items.length === 1 && page > 1) setPage(page - 1)
      else await load()
    } catch (e) { toast(e instanceof ApiError ? e.message : 'Delete failed', 'error') }
    finally { setBusy(false) }
  }

  const items = data?.items ?? []
  const pages = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1
  const votes = items.reduce((s, p) => s + (p.results?.total ?? 0), 0)
  const openCount = items.filter((p) => p.status === 'open').length
  const suffix = pages > 1 ? ' (this page)' : ''
  const first = user?.name.trim().split(' ')[0] ?? ''

  return (
    <div className="db">
      <header className="db-head">
        <div>
          <h1>Hi{first ? `, ${first}` : ''} <span className="lp-grad">👋</span></h1>
          <p className="db-sub">Here are your polls. Open one to watch votes arrive live.</p>
        </div>
        <Link className="btn btn-lg lp-btn" to="/polls/new">+ New poll</Link>
      </header>

      {data && data.total > 0 && (
        <div className="db-stats">
          <div className="db-stat"><strong>{fmtNumber(data.total)}</strong><span>Total polls</span></div>
          <div className="db-stat"><strong>{fmtNumber(votes)}</strong><span>Votes{suffix}</span></div>
          <div className="db-stat"><strong>{fmtNumber(openCount)}</strong><span>Open now{suffix}</span></div>
        </div>
      )}

      {loading && !data && <Skeleton lines={4} />}
      {error && (
        <div className="db-empty">
          <p className="error" role="alert">{error}</p>
          <button className="btn" onClick={() => void load()}>Retry</button>
        </div>
      )}
      {data && data.items.length === 0 && !error && (
        <div className="db-empty">
          <div className="db-empty-icon" aria-hidden="true">📊</div>
          <h2>No polls yet</h2>
          <p className="muted">Create your first poll and share the link to start collecting live votes.</p>
          <Link className="btn btn-lg lp-btn" to="/polls/new">Create a poll</Link>
        </div>
      )}

      {items.length > 0 && (
        <ul className="db-grid">
          {items.map((p) => {
            const total = p.results?.total ?? 0
            const pct = percentages(p.options, p.results)
            const top = [...p.options]
              .sort((a, b) => (p.results?.counts[b.id] ?? 0) - (p.results?.counts[a.id] ?? 0))
              .slice(0, 3)
            const open = p.status === 'open'
            return (
              <li key={p.id}>
                <article className={`dc ${p.status}`}>
                  <div className="dc-head">
                    <span className={`dc-state ${p.status}`}><span className="dc-dot" aria-hidden="true" />{open ? 'Live' : 'Closed'}</span>
                    <span className="dc-date">{fmtDate(p.createdAt)}</span>
                  </div>
                  <h3 className="dc-title"><Link to={`/p/${p.id}`}>{p.question}</Link></h3>
                  <div className="dc-stat">
                    <strong>{fmtNumber(total)}</strong>
                    <span>{total === 1 ? 'vote' : 'votes'}</span>
                    <em>{p.options.length} options</em>
                  </div>
                  <ul className="dc-bars" aria-label="Top options">
                    {top.map((o, i) => (
                      <li key={o.id}>
                        <span className="dc-name">{o.text}</span>
                        <span className="dc-track"><span className={i === 0 && total > 0 ? 'lead' : ''} style={{ width: `${pct[o.id]}%` }} /></span>
                        <span className="dc-pct">{pct[o.id]}%</span>
                      </li>
                    ))}
                  </ul>
                  <div className="dc-actions">
                    <Link className="icon-btn" to={`/p/${p.id}`} aria-label="View live results" title="View"><EyeIcon /></Link>
                    <button className="icon-btn" aria-label="Copy link" title="Copy link"
                      onClick={async () => toast((await copyText(p.shareUrl)) ? 'Link copied' : 'Copy failed', 'success')}><ShareIcon /></button>
                    <button className="icon-btn icon-danger" aria-label="Delete poll" title="Delete" onClick={() => setToDelete(p)}><TrashIcon /></button>
                  </div>
                </article>
              </li>
            )
          })}
          <li>
            <Link className="dc dc-new" to="/polls/new"><span aria-hidden="true">+</span>Create new poll</Link>
          </li>
        </ul>
      )}

      {pages > 1 && (
        <nav className="row center" aria-label="Pagination">
          <button className="btn btn-ghost" disabled={page <= 1} onClick={() => setPage(page - 1)}>Previous</button>
          <span className="muted">Page {page} of {pages}</span>
          <button className="btn btn-ghost" disabled={page >= pages} onClick={() => setPage(page + 1)}>Next</button>
        </nav>
      )}
      <ConfirmDialog open={!!toDelete} busy={busy} title="Delete this poll?" confirmLabel="Delete poll"
        message="This permanently deletes the poll and all of its votes. This cannot be undone." onConfirm={() => void confirmDelete()} onCancel={() => setToDelete(null)} />
    </div>
  )
}