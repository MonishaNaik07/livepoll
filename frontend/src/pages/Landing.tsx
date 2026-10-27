import { Link } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'

const STEPS = [
  ['1', 'Create', 'Sign in, write a question and add 2–10 options.'],
  ['2', 'Share', 'Send the public link or QR code. No account needed to vote.'],
  ['3', 'Vote', 'Each visitor gets one vote, enforced on the server.'],
  ['4', 'Watch live', 'Results move the instant a vote lands. No refresh.'],
]

export default function Landing() {
  const { user } = useAuth()
  return (
    <>
      <section className="hero">
        <p className="eyebrow">Real-time polling</p>
        <h1>Ask a question. Watch the answers roll in.</h1>
        <p className="lead">Create a poll in seconds, share one link, and see results update live for everyone watching.</p>
        <div className="row center">
          <Link className="btn btn-lg" to={user ? '/polls/new' : '/signup'}>Create a poll</Link>
          {!user && <Link className="btn btn-lg btn-ghost" to="/login">Sign in</Link>}
        </div>
      </section>
      <section aria-labelledby="how">
        <h2 id="how" className="center-text">How it works</h2>
        <ol className="steps">
          {STEPS.map(([n, t, d]) => (
            <li key={n} className="card"><span className="step-n">{n}</span><h3>{t}</h3><p>{d}</p></li>
          ))}
        </ol>
      </section>
    </>
  )
}
