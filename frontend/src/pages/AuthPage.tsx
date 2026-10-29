import { useState, type FormEvent } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { LogoMark } from '../components/Icons'
import { useAuth } from '../contexts/AuthContext'
import { ApiError } from '../services/api'

export function validateAuth(mode: 'login' | 'signup', f: { name: string; email: string; password: string }) {
  const e: Record<string, string> = {}
  if (mode === 'signup' && (f.name.trim().length < 1 || f.name.trim().length > 60)) e.name = 'Enter your name (max 60 characters)'
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(f.email.trim())) e.email = 'Enter a valid email address'
  if (mode === 'signup' ? f.password.length < 8 || f.password.length > 72 : f.password.length === 0)
    e.password = mode === 'signup' ? 'Password must be 8–72 characters' : 'Enter your password'
  return e
}

const PERKS = [
  ['⚡', 'Votes appear live, with no refresh'],
  ['🔗', 'Share with one link or a QR code'],
  ['🛡️', 'One vote per person, enforced on the server'],
]

export default function AuthPage({ mode }: { mode: 'login' | 'signup' }) {
  const { user, login, signup } = useAuth()
  const nav = useNavigate()
  const from = (useLocation().state as { from?: string } | null)?.from ?? '/dashboard'
  const [f, setF] = useState({ name: '', email: '', password: '' })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)
  const [show, setShow] = useState(false)

  if (user) return <Navigate to={from} replace />

  const isLogin = mode === 'login'

  const submit = async (ev: FormEvent) => {
    ev.preventDefault()
    setFormError('')
    const v = validateAuth(mode, f)
    setErrors(v)
    if (Object.keys(v).length) return
    setBusy(true)
    try {
      if (isLogin) await login(f.email.trim(), f.password)
      else await signup(f.name.trim(), f.email.trim(), f.password)
      nav(from, { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.fields) setErrors(err.fields)
      setFormError(err instanceof ApiError ? err.message : 'Unexpected error')
    } finally {
      setBusy(false)
    }
  }

  const field = (id: 'name' | 'email' | 'password', label: string, type: string, auto: string, placeholder: string) => (
    <div className="field">
      <label htmlFor={id} className="label">{label}</label>
      <div className="au-input">
        <input id={id} name={id} type={id === 'password' && show ? 'text' : type} className="input" autoComplete={auto}
          placeholder={placeholder} value={f[id]} aria-invalid={!!errors[id]} aria-describedby={errors[id] ? `${id}-err` : undefined}
          onChange={(e) => setF({ ...f, [id]: e.target.value })} />
        {id === 'password' && (
          <button type="button" className="au-eye" aria-pressed={show} onClick={() => setShow((s) => !s)}>
            {show ? 'Hide' : 'Show'}
          </button>
        )}
      </div>
      {errors[id] && <p id={`${id}-err`} className="error">{errors[id]}</p>}
    </div>
  )

  return (
    <div className="au">
      <aside className="au-brand" aria-hidden="true">
        <span className="au-logo"><LogoMark /> LivePoll</span>
        <h2>{isLogin ? 'Good to see you again.' : 'Start your first live poll.'}</h2>
        <p>{isLogin ? 'Pick up where you left off and watch the votes roll in.' : 'It takes less than a minute to create a poll and share it.'}</p>
        <ul>
          {PERKS.map(([icon, text]) => <li key={text}><span>{icon}</span>{text}</li>)}
        </ul>
        <span className="au-blob au-blob-a" /><span className="au-blob au-blob-b" />
      </aside>

      <section className="au-card">
        <h1>{isLogin ? 'Welcome back' : 'Create your account'}</h1>
        <p className="au-sub">{isLogin ? 'Log in to manage your polls.' : 'Sign up to create and manage polls.'}</p>
        <form onSubmit={submit} noValidate>
          {!isLogin && field('name', 'Name', 'text', 'name', 'Your name')}
          {field('email', 'Email', 'email', 'email', 'you@example.com')}
          {field('password', 'Password', 'password', isLogin ? 'current-password' : 'new-password', isLogin ? 'Your password' : 'At least 8 characters')}
          {formError && <p className="error banner" role="alert">{formError}</p>}
          <button className="btn btn-block btn-lg lp-btn" type="submit" disabled={busy}>
            {busy ? 'Please wait…' : isLogin ? 'Log in' : 'Create account'}
          </button>
        </form>
        <p className="au-switch">
          {isLogin ? <>New here? <Link to="/signup">Create an account</Link></> : <>Already registered? <Link to="/login">Log in</Link></>}
        </p>
      </section>
    </div>
  )
}