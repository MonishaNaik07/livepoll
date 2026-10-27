import type { Poll, PollPage, Results, User } from '../types'

const BASE: string = import.meta.env.VITE_API_BASE_URL ?? ''

export class ApiError extends Error {
  status: number
  code: string
  fields?: Record<string, string>
  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message)
    this.status = status
    this.code = code
    this.fields = fields
  }
}

async function raw(method: string, path: string, body?: unknown): Promise<Response> {
  try {
    return await fetch(BASE + path, {
      method,
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'livepoll' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network', 'Cannot reach the server. Check your connection and try again.')
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await raw(method, path, body)
  if (res.status === 204) return undefined as T
  const json = await res.json().catch(() => null)
  if (!res.ok) {
    const e = json?.error
    throw new ApiError(res.status, e?.code ?? 'error', e?.message ?? 'Something went wrong', e?.fields)
  }
  return json.data as T
}

export interface CreatePollBody {
  question: string
  description?: string
  options: string[]
  expiresAt?: string
}

export const api = {
  signup: (b: { name: string; email: string; password: string }) => request<User>('POST', '/api/auth/signup', b),
  login: (b: { email: string; password: string }) => request<User>('POST', '/api/auth/login', b),
  logout: () => request<unknown>('POST', '/api/auth/logout'),
  me: () => request<User>('GET', '/api/auth/me'),
  createPoll: (b: CreatePollBody) => request<Poll>('POST', '/api/polls', b),
  listPolls: (page: number, pageSize = 9) => request<PollPage>('GET', `/api/polls?page=${page}&pageSize=${pageSize}`),
  getPoll: (id: string) => request<Poll>('GET', `/api/polls/${encodeURIComponent(id)}`),
  results: (id: string) => request<Results>('GET', `/api/polls/${encodeURIComponent(id)}/results`),
  vote: (id: string, optionId: string) =>
    request<{ optionId: string; results: Results }>('POST', `/api/polls/${encodeURIComponent(id)}/votes`, { optionId }),
  close: (id: string) => request<Poll>('POST', `/api/polls/${encodeURIComponent(id)}/close`),
  reopen: (id: string) => request<Poll>('POST', `/api/polls/${encodeURIComponent(id)}/reopen`),
  remove: (id: string) => request<void>('DELETE', `/api/polls/${encodeURIComponent(id)}`),
  async exportCsv(id: string): Promise<Blob> {
    const res = await raw('GET', `/api/polls/${encodeURIComponent(id)}/export`)
    if (!res.ok) throw new ApiError(res.status, 'export_failed', 'Could not export results')
    return res.blob()
  },
}

export function wsUrl(pollId: string): string {
  const base: string =
    import.meta.env.VITE_WS_BASE_URL || `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}`
  return `${base}/api/ws/polls/${encodeURIComponent(pollId)}`
}
