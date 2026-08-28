import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Api, Blog, User } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { UserProfile } from './UserProfile'

const user: User = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  bio: 'Writes things.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}
const mine: Blog = {
  id: 'p1',
  ownerId: 'uid-1',
  authorUsername: 'calm-smiling-kestrel',
  title: 'Mine',
  content: 'hello',
  visibility: 'public',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
}
const theirs: Blog = {
  ...mine,
  id: 'p2',
  ownerId: 'uid-2',
  authorUsername: 'bold-leaping-lynx',
  title: 'Not mine',
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: vi.fn().mockResolvedValue([mine, theirs]),
    getBlog: vi.fn(),
    createBlog: vi.fn(),
    updateBlog: vi.fn(),
    deleteBlog: vi.fn(),
    getUser: vi.fn().mockResolvedValue(user),
    getCurrentUser: vi.fn(),
    putUser: vi.fn(),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('UserProfile', () => {
  it("shows the profile header and only that author's posts", async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi() },
      route: '/profile/calm-smiling-kestrel',
      path: '/profile/:username',
    })

    expect(await screen.findByRole('heading', { name: 'calm-smiling-kestrel' })).toBeInTheDocument()
    expect(screen.getByText('Writes things.')).toBeInTheDocument()
    expect(screen.getByText('Mine')).toBeInTheDocument()
    expect(screen.queryByText('Not mine')).not.toBeInTheDocument()
  })

  // An author with no profile holds no username, so no URL reaches this page for them: a lookup
  // that misses means the name is genuinely unclaimed, not that the author is nameless.
  it('reports an unclaimed username as no such user', async () => {
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new Error('not found')) })
    renderWithApp(<UserProfile />, {
      context: { api },
      route: '/profile/nobody-here-at-all',
      path: '/profile/:username',
    })

    expect(await screen.findByText('No such user.')).toBeInTheDocument()
  })

  // The Edit link belongs to the profile's owner, never to a signed-out visitor whose own profile
  // is also absent.
  it('offers Edit profile only to the owner', async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi(), profile: user },
      route: '/profile/calm-smiling-kestrel',
      path: '/profile/:username',
    })
    expect(await screen.findByRole('link', { name: 'Edit profile' })).toBeInTheDocument()
  })

  it('hides Edit profile from a signed-out visitor', async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi() },
      route: '/profile/calm-smiling-kestrel',
      path: '/profile/:username',
    })

    expect(await screen.findByRole('heading', { name: 'calm-smiling-kestrel' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Edit profile' })).not.toBeInTheDocument()
  })
})
