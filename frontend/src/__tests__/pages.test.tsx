import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import CreatePoll, { validatePoll } from '../pages/CreatePoll'
import PublicPoll from '../pages/PublicPoll'
import { validateAuth } from '../pages/AuthPage'
import { ToastProvider } from '../contexts/ToastContext'
import { ApiError } from '../services/api'
import type { Poll } from '../types'

const mocks = vi.hoisted(() => ({
  createPoll: vi.fn(), getPoll: vi.fn(), vote: vi.fn(), results: vi.fn(),
}))
vi.mock('../services/api', async (orig) => {
  const actual = await orig<typeof import('../services/api')>()
  return { ...actual, wsUrl: () => 'ws://test', api: { ...actual.api, ...mocks } }
})

class IdleWS { onopen = null; onmessage = null; onclose = null; onerror = null; close() {} }
beforeEach(() => { vi.stubGlobal('WebSocket', IdleWS); Object.values(mocks).forEach((m) => m.mockReset()); mocks.results.mockRejectedValue(new Error('x')) })
afterEach(() => vi.unstubAllGlobals())

describe('validation helpers', () => {
  it('validatePoll mirrors backend rules', () => {
    expect(validatePoll('hi', ['a', 'b'], '').question).toBeTruthy()
    expect(validatePoll('Good question', ['a', ''], '').options).toBeTruthy()
    expect(validatePoll('Good question', ['a', 'A'], '').options).toMatch(/unique/)
    expect(validatePoll('Good question', ['a', 'b'], '')).toEqual({})
  })
  it('validateAuth', () => {
    expect(validateAuth('signup', { name: '', email: 'x', password: 'short' })).toHaveProperty('email')
    expect(validateAuth('login', { name: '', email: 'a@b.co', password: 'x' })).toEqual({})
  })
})

describe('CreatePoll', () => {
  const setup = () => render(<MemoryRouter><ToastProvider><CreatePoll /></ToastProvider></MemoryRouter>)

  it('blocks submit and shows errors for empty input without calling the API', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: 'Create poll' }))
    expect(await screen.findByText(/Question must be/)).toBeInTheDocument()
    expect(mocks.createPoll).not.toHaveBeenCalled()
  })

  it('enforces option limits (min 2, max 10)', async () => {
    setup()
    expect(screen.getByRole('button', { name: 'Remove option 1' })).toBeDisabled()
    for (let i = 0; i < 8; i++) await userEvent.click(screen.getByRole('button', { name: '+ Add option' }))
    expect(screen.getByRole('button', { name: '+ Add option' })).toBeDisabled()
    expect(screen.getAllByLabelText(/^Option \d+$/)).toHaveLength(10)
  })

  it('submits trimmed values and shows the share link', async () => {
    mocks.createPoll.mockResolvedValue({ id: 'abc', question: 'Tabs or spaces?', shareUrl: 'http://x/p/abc', options: [], status: 'open' })
    setup()
    await userEvent.type(screen.getByLabelText('Question'), '  Tabs or spaces?  ')
    await userEvent.type(screen.getByLabelText('Option 1'), ' Tabs ')
    await userEvent.type(screen.getByLabelText('Option 2'), 'Spaces')
    await userEvent.click(screen.getByRole('button', { name: 'Create poll' }))
    await waitFor(() => expect(mocks.createPoll).toHaveBeenCalledWith(expect.objectContaining({ question: 'Tabs or spaces?', options: ['Tabs', 'Spaces'] })))
    expect(await screen.findByDisplayValue('http://x/p/abc')).toBeInTheDocument()
  })

  it('surfaces server validation errors', async () => {
    mocks.createPoll.mockRejectedValue(new ApiError(422, 'validation_failed', 'Invalid input', { question: 'Server says no' }))
    setup()
    await userEvent.type(screen.getByLabelText('Question'), 'Valid question')
    await userEvent.type(screen.getByLabelText('Option 1'), 'a')
    await userEvent.type(screen.getByLabelText('Option 2'), 'b')
    await userEvent.click(screen.getByRole('button', { name: 'Create poll' }))
    expect(await screen.findByText('Server says no')).toBeInTheDocument()
  })
})

describe('PublicPoll voting', () => {
  const poll: Poll = {
    id: 'p1', question: 'Best editor?', status: 'open', createdAt: new Date().toISOString(), isOwner: false, shareUrl: 'http://x/p/p1',
    options: [{ id: 'o1', text: 'Vim' }, { id: 'o2', text: 'Emacs' }],
    results: { pollId: 'p1', status: 'open', total: 0, counts: { o1: 0, o2: 0 }, seq: 0 },
  }
  const setup = () => render(
    <MemoryRouter initialEntries={['/p/p1']}><ToastProvider><Routes><Route path="/p/:id" element={<PublicPoll />} /></Routes></ToastProvider></MemoryRouter>)

  it('requires a choice, submits the vote, then shows updated results', async () => {
    mocks.getPoll.mockResolvedValue(poll)
    mocks.vote.mockResolvedValue({ optionId: 'o2', results: { pollId: 'p1', status: 'open', total: 1, counts: { o1: 0, o2: 1 }, seq: 1 } })
    setup()
    await screen.findByText('Best editor?')
    expect(screen.getByText('No votes yet. Be the first!')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Submit vote' }))
    expect(await screen.findByText('Choose an option first')).toBeInTheDocument()
    expect(mocks.vote).not.toHaveBeenCalled()
    await userEvent.click(screen.getByLabelText('Emacs'))
    await userEvent.click(screen.getByRole('button', { name: 'Submit vote' }))
    expect(await screen.findByText(/Your vote is in/)).toBeInTheDocument()
    expect(screen.getByText('1 vote total')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Submit vote' })).not.toBeInTheDocument()
  })

  it('handles a server-side duplicate-vote rejection', async () => {
    mocks.getPoll.mockResolvedValue(poll)
    mocks.vote.mockRejectedValue(new ApiError(409, 'already_voted', 'You have already voted on this poll'))
    setup()
    await userEvent.click(await screen.findByLabelText('Vim'))
    await userEvent.click(screen.getByRole('button', { name: 'Submit vote' }))
    expect(await screen.findByText(/already voted|Your vote is in/)).toBeInTheDocument()
  })

  it('hides the vote form on a closed poll', async () => {
    mocks.getPoll.mockResolvedValue({ ...poll, status: 'closed', results: { ...poll.results!, status: 'closed' } })
    setup()
    expect(await screen.findByText(/This poll is closed/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Submit vote' })).not.toBeInTheDocument()
  })

  it('shows a not-found state', async () => {
    mocks.getPoll.mockRejectedValue(new ApiError(404, 'not_found', 'Resource not found'))
    setup()
    expect(await screen.findByText('Poll not found')).toBeInTheDocument()
  })
})
