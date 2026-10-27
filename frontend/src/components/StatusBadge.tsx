import type { PollStatus } from '../types'
export const StatusBadge = ({ status }: { status: PollStatus }) => (
  <span className={`badge badge-${status}`}>{status === 'open' ? 'Open' : 'Closed'}</span>
)
