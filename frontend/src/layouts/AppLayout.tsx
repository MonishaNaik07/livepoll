import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'

export function AppLayout() {
  const { user, logout } = useAuth()
  const nav = useNavigate()
  const [open, setOpen] = useState(false)
  const [dark, setDark] = useState(() => localStorage.getItem('theme') === 'dark')
  useEffect(() => {
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    try { localStorage.setItem('theme', dark ? 'dark' : 'light') } catch { /* storage unavailable */ }
  }, [dark])

  const close = () => setOpen(false)
  return (
    <>
      <a href="#main" className="skip">Skip to content</a>
      <header className="nav">
        <div className="nav-inner">
          <Link to="/" className="brand" onClick={close}><span className="logo" aria-hidden="true" />LivePoll</Link>
          <button className="btn btn-ghost menu" aria-expanded={open} aria-controls="nav-links" onClick={() => setOpen((o) => !o)}>Menu</button>
          <nav id="nav-links" className={`links${open ? ' open' : ''}`} aria-label="Main">
            {user ? (
              <>
                <NavLink to="/dashboard" onClick={close}>Dashboard</NavLink>
                <NavLink to="/polls/new" onClick={close}>New poll</NavLink>
                <button className="btn btn-ghost" onClick={async () => { close(); await logout(); nav('/') }}>Log out</button>
              </>
            ) : (
              <>
                <NavLink to="/login" onClick={close}>Log in</NavLink>
                <Link to="/signup" className="btn" onClick={close}>Sign up</Link>
              </>
            )}
            <button className="btn btn-ghost" aria-pressed={dark} onClick={() => setDark((d) => !d)} aria-label="Toggle dark mode">{dark ? '☀️' : '🌙'}</button>
          </nav>
        </div>
      </header>
      <main id="main" className="container"><Outlet /></main>
      <footer className="footer">LivePoll · React · Go (Gin) · MongoDB · Redis</footer>
    </>
  )
}
