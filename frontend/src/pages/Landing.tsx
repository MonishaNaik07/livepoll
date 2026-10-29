import { Link } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'

const FEATURES = [
  ['⚡', 'Truly live', 'Votes appear on every screen the moment they land. No refresh, ever.'],
  ['🔗', 'One link to share', 'Send a link or QR code. Voters need no account.'],
  ['🛡️', 'One vote each', 'Duplicate votes are blocked on the server, not just in the browser.'],
  ['⏱️', 'Auto-close', 'Set an expiry and the poll closes itself, live, for everyone.'],
  ['📥', 'Export results', 'Download your results as a CSV with one click.'],
  ['🌙', 'Light & dark', 'A clean interface that works on desktop, tablet and phone.'],
]

const STEPS = [
  ['1', 'Create', 'Sign in, write a question and add 2–10 options.'],
  ['2', 'Share', 'Copy the public link or show the QR code.'],
  ['3', 'Vote', 'Anyone with the link picks an option. One vote per person.'],
  ['4', 'Watch live', 'Bars move in real time as the votes come in.'],
]

const STACK = ['React', 'Go · Gin', 'MongoDB', 'Redis', 'WebSockets']

const DEMO = [
  { label: 'Pizza', pct: 52, lead: true },
  { label: 'Burgers', pct: 31, lead: false },
  { label: 'Tacos', pct: 17, lead: false },
]

export default function Landing() {
  const { user } = useAuth()
  const cta = user ? '/polls/new' : '/signup'
  return (
    <div className="lp">
      <section className="lp-hero">
        <div className="lp-copy">
          <span className="lp-eyebrow"><span className="lp-pulse" aria-hidden="true" /> Real-time polling</span>
          <h1>Ask a question.<br />Watch the answers <span className="lp-grad">roll in.</span></h1>
          <p className="lp-lead">
            Create a poll in seconds, share one link, and see every vote appear live for everyone watching.
          </p>
          <div className="lp-cta">
            <Link className="btn btn-lg lp-btn" to={cta}>{user ? 'Create a poll' : 'Get started free'}</Link>
            {user
              ? <Link className="btn btn-lg btn-ghost" to="/dashboard">My dashboard</Link>
              : <Link className="btn btn-lg btn-ghost" to="/login">Sign in</Link>}
          </div>
        </div>

        <div className="lp-preview" aria-hidden="true">
          <div className="lp-card">
            <div className="lp-card-top">
              <span className="lp-live"><span className="lp-pulse" /> Live</span>
              <span className="lp-example">Example</span>
            </div>
            <p className="lp-q">What should we order for lunch?</p>
            {DEMO.map((d, i) => (
              <div className="lp-row" key={d.label}>
                <div className="lp-row-head"><span>{d.label}</span><strong>{d.pct}%</strong></div>
                <div className="lp-track"><span className={d.lead ? 'lead' : ''} style={{ width: `${d.pct}%`, animationDelay: `${i * 0.15}s` }} /></div>
              </div>
            ))}
            <p className="lp-foot">Illustration of how a poll looks</p>
          </div>
          <span className="lp-blob lp-blob-a" />
          <span className="lp-blob lp-blob-b" />
        </div>
      </section>

      <section className="lp-stack" aria-label="Built with">
        <span>Built with</span>
        {STACK.map((s) => <b key={s}>{s}</b>)}
      </section>

      <section className="lp-section" aria-labelledby="features">
        <h2 id="features">Everything you need, nothing you don’t</h2>
        <p className="lp-sub">Simple to start, solid underneath.</p>
        <ul className="lp-features">
          {FEATURES.map(([icon, title, text]) => (
            <li key={title} className="lp-feature">
              <span className="lp-icon" aria-hidden="true">{icon}</span>
              <h3>{title}</h3>
              <p>{text}</p>
            </li>
          ))}
        </ul>
      </section>

      <section className="lp-section" aria-labelledby="how">
        <h2 id="how">How it works</h2>
        <p className="lp-sub">From idea to live results in under a minute.</p>
        <ol className="lp-steps">
          {STEPS.map(([n, title, text]) => (
            <li key={n} className="lp-step">
              <span className="lp-num">{n}</span>
              <h3>{title}</h3>
              <p>{text}</p>
            </li>
          ))}
        </ol>
      </section>

      <section className="lp-final">
        <h2>Ready to see votes land in real time?</h2>
        <p>Create your first poll in less than a minute.</p>
        <Link className="btn btn-lg lp-btn-light" to={cta}>{user ? 'Create a poll' : 'Create your free account'}</Link>
      </section>
    </div>
  )
}