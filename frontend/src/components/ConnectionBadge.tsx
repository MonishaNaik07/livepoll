import type { ConnStatus } from '../types'

const LABEL: Record<ConnStatus, string> = {
  connecting: 'Connecting…',
  live: 'Live',
  reconnecting: 'Reconnecting…',
  offline: 'Offline',
}

export function ConnectionBadge({ status }: { status: ConnStatus }) {
  return (
    <span className={`conn conn-${status}`} role="status" aria-live="polite">
      <span className="dot" aria-hidden="true" />
      {LABEL[status]}
      {status !== 'live' && status !== 'connecting' && <span className="sr-only"> – showing last known results</span>}
    </span>
  )
}
