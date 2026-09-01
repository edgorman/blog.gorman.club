import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { AppContext, type AppContextValue } from '../context/AppContext'
import { ApiError, type Api, type PageReactions } from '../lib/api'
import { fakeAppContext } from '../testUtils'
import { useReactions } from './useReactions'

const page: PageReactions = {
  post: [{ emoji: '👍', count: 2, reacted: false }],
  comments: { cmt1: [{ emoji: '🔥', count: 1, reacted: true }] },
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    getReactions: vi.fn().mockResolvedValue(page),
    addReaction: vi.fn().mockResolvedValue([{ emoji: '👍', count: 3, reacted: true }]),
    removeReaction: vi.fn().mockResolvedValue([]),
    ...overrides,
  } as unknown as Api
}

function renderUseReactions(api: Api) {
  const context: AppContextValue = fakeAppContext({ api })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <MemoryRouter>
      <AppContext.Provider value={context}>{children}</AppContext.Provider>
    </MemoryRouter>
  )
  return renderHook(() => useReactions('hello-world'), { wrapper })
}

describe('useReactions', () => {
  it('loads the whole page’s reactions in one request', async () => {
    const api = fakeApi()
    const { result } = renderUseReactions(api)

    await waitFor(() => expect(result.current.countsFor()).toHaveLength(1))
    expect(api.getReactions).toHaveBeenCalledTimes(1)
    expect(api.getReactions).toHaveBeenCalledWith('hello-world')
    expect(result.current.countsFor('cmt1')[0].emoji).toBe('🔥')
    // A comment nobody has reacted to has no entry, and reads as an empty bar rather than crashing.
    expect(result.current.countsFor('cmt404')).toEqual([])
  })

  // The API has no toggle of its own, deliberately, so the hook decides which way to write from
  // what is stored - and a click on a reaction the reader is not in adds theirs.
  it('adds a reaction the reader is not yet in', async () => {
    const api = fakeApi()
    const { result } = renderUseReactions(api)
    await waitFor(() => expect(result.current.countsFor()).toHaveLength(1))

    act(() => result.current.toggle('👍'))

    await waitFor(() => expect(api.addReaction).toHaveBeenCalledWith('hello-world', '👍', undefined))
    expect(api.removeReaction).not.toHaveBeenCalled()
    // The response replaces the count, so another reader's click that arrived in between is picked
    // up rather than overwritten by an optimistic guess.
    await waitFor(() => expect(result.current.countsFor()[0].count).toBe(3))
  })

  it('takes back a reaction the reader is already in', async () => {
    const api = fakeApi()
    const { result } = renderUseReactions(api)
    await waitFor(() => expect(result.current.countsFor('cmt1')).toHaveLength(1))

    act(() => result.current.toggle('🔥', 'cmt1'))

    await waitFor(() => expect(api.removeReaction).toHaveBeenCalledWith('hello-world', '🔥', 'cmt1'))
    await waitFor(() => expect(result.current.countsFor('cmt1')).toHaveLength(0))
  })

  it('reports a reaction it could not save', async () => {
    const api = fakeApi({ addReaction: vi.fn().mockRejectedValue(new ApiError(429, 'slow down')) })
    const { result } = renderUseReactions(api)
    await waitFor(() => expect(result.current.countsFor()).toHaveLength(1))

    act(() => result.current.toggle('🎉'))

    await waitFor(() => expect(result.current.error).toBe('slow down'))
  })

  // The post and its comments are the point of the page; a bar nobody can load is not worth an
  // error above them.
  it('leaves the page readable when the reactions cannot be loaded', async () => {
    const api = fakeApi({ getReactions: vi.fn().mockRejectedValue(new ApiError(500, 'nope')) })
    const { result } = renderUseReactions(api)

    await waitFor(() => expect(api.getReactions).toHaveBeenCalled())
    expect(result.current.countsFor()).toEqual([])
    expect(result.current.error).toBeNull()
  })
})
