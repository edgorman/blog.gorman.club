import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Api, Blog, User } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { UserProfile } from './UserProfile'

const user: User = {
  id: 'uid-1',
  displayName: 'Mia Gorman',
  bio: 'Writes things.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}
const mine: Blog = {
  id: 'p1',
  ownerId: 'uid-1',
  title: 'Mine',
  content: 'hello',
  visibility: 'public',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
}
const theirs: Blog = { ...mine, id: 'p2', ownerId: 'uid-2', title: 'Not mine' }

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: vi.fn().mockResolvedValue([mine, theirs]),
    getBlog: vi.fn(),
    createBlog: vi.fn(),
    updateBlog: vi.fn(),
    deleteBlog: vi.fn(),
    getUser: vi.fn().mockResolvedValue(user),
    putUser: vi.fn(),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('UserProfile', () => {
  it("shows the profile header and only that author's posts", async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi() },
      route: '/profile/uid-1',
      path: '/profile/:id',
    })

    expect(await screen.findByText('Mia Gorman')).toBeInTheDocument()
    expect(screen.getByText('Writes things.')).toBeInTheDocument()
    expect(screen.getByText('Mine')).toBeInTheDocument()
    expect(screen.queryByText('Not mine')).not.toBeInTheDocument()
  })

  it('falls back to a resolved name and a sign-in hint when the profile lookup fails', async () => {
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new Error('unauthorized')) })
    renderWithApp(<UserProfile />, {
      context: { api, resolveAuthorName: () => Promise.resolve('uid-1234…') },
      route: '/profile/uid-1',
      path: '/profile/:id',
    })

    expect(await screen.findByText('uid-1234…')).toBeInTheDocument()
    expect(screen.getByText(/Sign in to see this author's full profile/)).toBeInTheDocument()
  })
})
