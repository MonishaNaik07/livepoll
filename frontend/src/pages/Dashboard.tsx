import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { Skeleton } from '../components/Skeleton'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../contexts/ToastContext'
import { api, ApiError } from '../services/api'
import type { Poll, PollPage } from '../types'
import { copyText } from '../utils/clipboard'
import { fmtDate, fmtNumber } from '../utils/results'

export default function Dashboard() {
  const toast = useToast()
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

  const toggle = async (p: Poll) => {
    try {
      p.status === 'open' ? await api.close(p.id) : await api.reopen(p.id)
      toast(p.status === 'open' ? 'Poll closed' : 'Poll reopened', 'success')
      await load()
    } catch (e) { toast(e instanceof ApiError ? e.message : 'Action failed', 'error') }
  }

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

  const pages = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1
  return (
    <>
      <div className="row between wrap">
        <h1>Your polls</h1>
        <Link className="btn" to="/polls/new">+ New poll</Link>
      </div>
      {loading && !data && <Skeleton lines={4} />}
      {error && <div className="card"><p className="error" role="alert">{error}</p><button className="btn" onClick={() => void load()}>Retry</button></div>}
      {data && data.items.length === 0 && !error && (
        <div className="card empty"><h2>No polls yet</h2><p className="muted">Create your first poll and share the link to start collecting live votes.</p><Link className="btn" to="/polls/new">Create a poll</Link></div>
      )}
      <ul className="grid">
        {data?.items.map((p) => (
          <li key={p.id} className="card poll-card">
            <div className="row between"><StatusBadge status={p.status} /><span className="muted">{fmtDate(p.createdAt)}</span></div>
            <h3><Link to={`/p/${p.id}`}>{p.question}</Link></h3>
            <p className="muted">{fmtNumber(p.results?.total ?? 0)} {(p.results?.total ?? 0) === 1 ? 'vote' : 'votes'}</p>
            <div className="row wrap">
              <Link className="btn btn-sm" to={`/p/${p.id}`}>View live</Link>
              <button className="btn btn-sm btn-ghost" onClick={async () => toast((await copyText(p.shareUrl)) ? 'Link copied' : 'Copy failed', 'success')}>Copy link</button>
              <button className="btn btn-sm btn-ghost" onClick={() => void toggle(p)}>{p.status === 'open' ? 'Close' : 'Reopen'}</button>
              <button className="btn btn-sm btn-ghost danger" onClick={() => setToDelete(p)}>Delete</button>
            </div>
          </li>
        ))}
      </ul>
      {pages > 1 && (
        <nav className="row center" aria-label="Pagination">
          <button className="btn btn-ghost" disabled={page <= 1} onClick={() => setPage(page - 1)}>Previous</button>
          <span className="muted">Page {page} of {pages}</span>
          <button className="btn btn-ghost" disabled={page >= pages} onClick={() => setPage(page + 1)}>Next</button>
        </nav>
      )}
      <ConfirmDialog open={!!toDelete} busy={busy} title="Delete this poll?" confirmLabel="Delete poll"
        message="This permanently deletes the poll and all of its votes. This cannot be undone." onConfirm={() => void confirmDelete()} onCancel={() => setToDelete(null)} />
    </>
  )
}
