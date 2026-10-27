import { Route, Routes, Link } from 'react-router-dom'
import { ProtectedRoute } from './components/ProtectedRoute'
import { AppLayout } from './layouts/AppLayout'
import AuthPage from './pages/AuthPage'
import CreatePoll from './pages/CreatePoll'
import Dashboard from './pages/Dashboard'
import Landing from './pages/Landing'
import PublicPoll from './pages/PublicPoll'

export default function App() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route path="/" element={<Landing />} />
        <Route path="/login" element={<AuthPage mode="login" />} />
        <Route path="/signup" element={<AuthPage mode="signup" />} />
        <Route path="/dashboard" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
        <Route path="/polls/new" element={<ProtectedRoute><CreatePoll /></ProtectedRoute>} />
        <Route path="/p/:id" element={<PublicPoll />} />
        <Route path="*" element={<section className="card narrow center-text"><h1>Page not found</h1><Link className="btn" to="/">Go home</Link></section>} />
      </Route>
    </Routes>
  )
}
