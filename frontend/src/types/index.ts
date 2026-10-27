export interface User { id: string; name: string; email: string }
export interface PollOption { id: string; text: string }
export type PollStatus = 'open' | 'closed'

export interface Results {
  pollId: string
  status: PollStatus
  total: number
  counts: Record<string, number>
  seq: number
}

export interface Poll {
  id: string
  question: string
  description?: string
  options: PollOption[]
  status: PollStatus
  expiresAt?: string
  createdAt: string
  isOwner: boolean
  shareUrl: string
  results?: Results
  hasVoted?: boolean
  myOptionId?: string
}

export interface PollPage { items: Poll[]; total: number; page: number; pageSize: number }
export type ConnStatus = 'connecting' | 'live' | 'reconnecting' | 'offline'
