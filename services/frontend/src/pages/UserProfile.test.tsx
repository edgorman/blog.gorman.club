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
  slug: 'mine',
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
  slug: 'not-mine',
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

  // Editing is reached from the account panel alone, so the owner's own profile page carries no
  // Edit link either.
  it('leaves the Edit profile link to the account panel', async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi(), profile: user },
      route: '/profile/calm-smiling-kestrel',
      path: '/profile/:username',
    })

    expect(await screen.findByRole('heading', { name: 'calm-smiling-kestrel' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Edit profile' })).not.toBeInTheDocument()
  })

  // The backend folds case when resolving a username, so a link that differs only in case has to
  // behave identically here - header and posts alike.
  it('matches the author regardless of the case in the URL', async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi(), profile: user },
      route: '/profile/CALM-Smiling-Kestrel',
      path: '/profile/:username',
    })

    expect(await screen.findByRole('heading', { name: 'calm-smiling-kestrel' })).toBeInTheDocument()
    expect(screen.getByText('Mine')).toBeInTheDocument()
    expect(screen.queryByText('Not mine')).not.toBeInTheDocument()
  })
})
