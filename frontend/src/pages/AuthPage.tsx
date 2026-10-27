import { useState, type FormEvent } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
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

export default function AuthPage({ mode }: { mode: 'login' | 'signup' }) {
  const { user, login, signup } = useAuth()
  const nav = useNavigate()
  const from = (useLocation().state as { from?: string } | null)?.from ?? '/dashboard'
  const [f, setF] = useState({ name: '', email: '', password: '' })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  if (user) return <Navigate to={from} replace />

  const submit = async (ev: FormEvent) => {
    ev.preventDefault()
    setFormError('')
    const v = validateAuth(mode, f)
    setErrors(v)
    if (Object.keys(v).length) return
    setBusy(true)
    try {
      if (mode === 'login') await login(f.email.trim(), f.password)
      else await signup(f.name.trim(), f.email.trim(), f.password)
      nav(from, { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.fields) setErrors(err.fields)
      setFormError(err instanceof ApiError ? err.message : 'Unexpected error')
    } finally {
      setBusy(false)
    }
  }

  const field = (id: 'name' | 'email' | 'password', label: string, type: string, auto: string) => (
    <div className="field">
      <label htmlFor={id} className="label">{label}</label>
      <input id={id} name={id} type={type} className="input" autoComplete={auto} value={f[id]}
        aria-invalid={!!errors[id]} aria-describedby={errors[id] ? `${id}-err` : undefined}
        onChange={(e) => setF({ ...f, [id]: e.target.value })} />
      {errors[id] && <p id={`${id}-err`} className="error">{errors[id]}</p>}
    </div>
  )

  return (
    <section className="card narrow">
      <h1>{mode === 'login' ? 'Welcome back' : 'Create your account'}</h1>
      <form onSubmit={submit} noValidate>
        {mode === 'signup' && field('name', 'Name', 'text', 'name')}
        {field('email', 'Email', 'email', 'email')}
        {field('password', 'Password', 'password', mode === 'login' ? 'current-password' : 'new-password')}
        {formError && <p className="error banner" role="alert">{formError}</p>}
        <button className="btn btn-block" type="submit" disabled={busy}>{busy ? 'Please wait…' : mode === 'login' ? 'Log in' : 'Sign up'}</button>
      </form>
      <p className="muted center-text">
        {mode === 'login' ? <>New here? <Link to="/signup">Create an account</Link></> : <>Already registered? <Link to="/login">Log in</Link></>}
      </p>
    </section>
  )
}
