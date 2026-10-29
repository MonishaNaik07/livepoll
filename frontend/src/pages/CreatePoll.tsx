import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { SharePanel } from '../components/SharePanel'
import { api, ApiError } from '../services/api'
import type { Poll } from '../types'

export const MIN_OPTIONS = 2
export const MAX_OPTIONS = 10

export function validatePoll(question: string, options: string[], expiresAt: string) {
  const e: Record<string, string> = {}
  const q = question.trim()
  if (q.length < 3 || q.length > 200) e.question = 'Question must be 3–200 characters'
  const texts = options.map((o) => o.trim())
  if (texts.some((t) => t.length === 0)) e.options = 'Fill in or remove empty options'
  else if (texts.some((t) => t.length > 100)) e.options = 'Options can be at most 100 characters'
  else if (new Set(texts.map((t) => t.toLowerCase())).size !== texts.length) e.options = 'Options must be unique'
  if (expiresAt && new Date(expiresAt).getTime() < Date.now() + 60_000) e.expiresAt = 'Expiry must be at least 1 minute from now'
  return e
}

export default function CreatePoll() {
  const [question, setQuestion] = useState('')
  const [description, setDescription] = useState('')
  const [options, setOptions] = useState(['', ''])
  const [expiresAt, setExpiresAt] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)
  const [created, setCreated] = useState<Poll | null>(null)

  const submit = async (ev: FormEvent) => {
    ev.preventDefault()
    setFormError('')
    const v = validatePoll(question, options, expiresAt)
    setErrors(v)
    if (Object.keys(v).length) return
    setBusy(true)
    try {
      setCreated(await api.createPoll({
        question: question.trim(), description: description.trim() || undefined, options: options.map((o) => o.trim()),
        expiresAt: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      }))
    } catch (err) {
      if (err instanceof ApiError && err.fields) setErrors(err.fields)
      setFormError(err instanceof ApiError ? err.message : 'Unexpected error')
    } finally {
      setBusy(false)
    }
  }

  if (created) {
    return (
      <section className="card narrow">
        <h1>Poll created 🎉</h1>
        <p className="lead-sm">{created.question}</p>
        <SharePanel url={created.shareUrl} />
        <div className="row wrap">
          <button className="btn btn-ghost" onClick={() => { setCreated(null); setQuestion(''); setDescription(''); setOptions(['', '']); setExpiresAt('') }}>Create another</button>
        </div>
      </section>
    )
  }

  return (
    <section className="card narrow">
      <h1>New poll</h1>
      <form onSubmit={submit} noValidate>
        <div className="field">
          <label className="label" htmlFor="q">Question</label>
          <input id="q" className="input" value={question} maxLength={200} onChange={(e) => setQuestion(e.target.value)} aria-invalid={!!errors.question} aria-describedby={errors.question ? 'q-err' : undefined} />
          {errors.question && <p id="q-err" className="error">{errors.question}</p>}
        </div>
        <div className="field">
          <label className="label" htmlFor="d">Description <span className="muted">(optional)</span></label>
          <textarea id="d" className="input" rows={2} maxLength={500} value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <fieldset className="field">
          <legend className="label">Options ({options.length}/{MAX_OPTIONS})</legend>
          {options.map((o, i) => (
            <div className="row" key={i}>
              <input className="input" aria-label={`Option ${i + 1}`} value={o} maxLength={100}
                onChange={(e) => setOptions(options.map((x, j) => (j === i ? e.target.value : x)))} />
              <button type="button" className="btn btn-ghost" aria-label={`Remove option ${i + 1}`}
                disabled={options.length <= MIN_OPTIONS} onClick={() => setOptions(options.filter((_, j) => j !== i))}>✕</button>
            </div>
          ))}
          {errors.options && <p className="error" role="alert">{errors.options}</p>}
          <button type="button" className="btn btn-ghost" disabled={options.length >= MAX_OPTIONS} onClick={() => setOptions([...options, ''])}>+ Add option</button>
        </fieldset>
        <div className="field">
          <label className="label" htmlFor="exp">Closes automatically <span className="muted">(optional)</span></label>
          <input id="exp" type="datetime-local" className="input" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} aria-invalid={!!errors.expiresAt} />
          {errors.expiresAt && <p className="error">{errors.expiresAt}</p>}
        </div>
        {formError && <p className="error banner" role="alert">{formError}</p>}
        <button className="btn btn-block" type="submit" disabled={busy}>{busy ? 'Creating…' : 'Create poll'}</button>
      </form>
    </section>
  )
}
