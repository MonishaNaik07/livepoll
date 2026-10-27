import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { ConnectionBadge } from '../components/ConnectionBadge'
import { ResultBars } from '../components/ResultBars'
import { SharePanel } from '../components/SharePanel'
import { Skeleton } from '../components/Skeleton'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../contexts/ToastContext'
import { usePollSocket } from '../hooks/usePollSocket'
import { api, ApiError } from '../services/api'
import type { Poll } from '../types'

export default function PublicPoll() {
  const { id = '' } = useParams()
  const nav = useNavigate()
  const toast = useToast()
  const [poll, setPoll] = useState<Poll | null>(null)
  const [error, setError] = useState<{ status: number; message: string } | null>(null)
  const [choice, setChoice] = useState('')
  const [voted, setVoted] = useState<string | undefined>()
  const [submitting, setSubmitting] = useState(false)
  const [voteError, setVoteError] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    setPoll(null); setError(null)
    api.getPoll(id).then((p) => { setPoll(p); if (p.hasVoted) setVoted(p.myOptionId) })
      .catch((e) => setError({ status: e instanceof ApiError ? e.status : 0, message: e instanceof ApiError ? e.message : 'Unexpected error' }))
  }, [id])

  const { results, status, push } = usePollSocket(poll ? poll.id : undefined, poll?.results)

  if (error) {
    return (
      <section className="card narrow center-text">
        <h1>{error.status === 404 || error.status === 400 ? 'Poll not found' : 'Something went wrong'}</h1>
        <p className="muted">{error.status === 404 || error.status === 400 ? 'This link may be wrong, or the poll was deleted.' : error.message}</p>
        <Link className="btn" to="/">Go home</Link>
      </section>
    )
  }
  if (!poll) return <section className="card narrow"><Skeleton lines={4} /></section>

  const live = results?.status ?? poll.status // status changes arrive over the socket in real time
  const canVote = live === 'open' && !voted

  const submit = async (ev: FormEvent) => {
    ev.preventDefault()
    if (!choice) { setVoteError('Choose an option first'); return }
    setSubmitting(true); setVoteError('')
    try {
      const r = await api.vote(poll.id, choice)
      setVoted(r.optionId)
      if (r.results.total > 0) push(r.results)
      toast('Vote recorded. Thanks!', 'success')
    } catch (e) {
      if (e instanceof ApiError && e.code === 'already_voted') { setVoted(choice); setVoteError('You have already voted on this poll.') }
      else setVoteError(e instanceof ApiError ? e.message : 'Could not submit your vote')
    } finally { setSubmitting(false) }
  }

  const ownerAction = async (fn: () => Promise<unknown>, ok: string) => {
    try { await fn(); toast(ok, 'success') } catch (e) { toast(e instanceof ApiError ? e.message : 'Action failed', 'error') }
  }
  const exportCsv = () => ownerAction(async () => {
    const url = URL.createObjectURL(await api.exportCsv(poll.id))
    const a = document.createElement('a'); a.href = url; a.download = `poll-${poll.id}.csv`; a.click(); URL.revokeObjectURL(url)
  }, 'CSV downloaded')

  return (
    <div className="stack">
      <section className="card narrow">
        <div className="row between wrap">
          <StatusBadge status={live} />
          <ConnectionBadge status={status} />
        </div>
        <h1>{poll.question}</h1>
        {poll.description && <p className="muted">{poll.description}</p>}

        {canVote ? (
          <form onSubmit={submit}>
            <fieldset className="options">
              <legend className="sr-only">Choose one option</legend>
              {poll.options.map((o) => (
                <label key={o.id} className={`option${choice === o.id ? ' selected' : ''}`}>
                  <input type="radio" name="option" value={o.id} checked={choice === o.id} onChange={() => { setChoice(o.id); setVoteError('') }} />
                  <span>{o.text}</span>
                </label>
              ))}
            </fieldset>
            {voteError && <p className="error banner" role="alert">{voteError}</p>}
            <button className="btn btn-block" type="submit" disabled={submitting}>{submitting ? 'Submitting…' : 'Submit vote'}</button>
          </form>
        ) : (
          <>
            {live === 'closed' && <p className="notice">This poll is closed. Final results are below.</p>}
            {voted && <p className="notice ok">✓ Your vote is in. Results update live below.</p>}
          </>
        )}
      </section>

      <section className="card narrow" aria-label="Live results">
        <h2>Live results</h2>
        <ResultBars options={poll.options} results={results} highlightId={voted} />
        {status !== 'live' && <p className="muted small">Live connection {status === 'offline' ? 'lost – you appear to be offline' : 'interrupted'}. Results refresh automatically while we reconnect.</p>}
      </section>

      {poll.isOwner && (
        <section className="card narrow" aria-label="Manage poll">
          <h2>Manage poll</h2>
          <SharePanel url={poll.shareUrl} />
          <div className="row wrap">
            {live === 'open'
              ? <button className="btn btn-ghost" onClick={() => void ownerAction(() => api.close(poll.id), 'Poll closed')}>Close poll</button>
              : <button className="btn btn-ghost" onClick={() => void ownerAction(() => api.reopen(poll.id), 'Poll reopened')}>Reopen poll</button>}
            <button className="btn btn-ghost" onClick={() => void exportCsv()}>Export CSV</button>
            <button className="btn btn-ghost danger" onClick={() => setConfirmDelete(true)}>Delete</button>
          </div>
        </section>
      )}
      <ConfirmDialog open={confirmDelete} title="Delete this poll?" confirmLabel="Delete poll"
        message="This permanently deletes the poll and all of its votes. This cannot be undone."
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => void ownerAction(async () => { await api.remove(poll.id); nav('/dashboard') }, 'Poll deleted')} />
    </div>
  )
}
