import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Api, Blog, BlogPage, CurrentUser, ListBlogsParams, User } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { UserProfile } from './UserProfile'

const user: User = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  bio: 'Writes things.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}
// The caller's own profile carries what their account may do as well as who they are; a public
// profile (`user` above) deliberately does not.
const profile: CurrentUser = { ...user, assistantEnabled: false }
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

// The real backend does the owner-scoping `listBlogs({ ownerId })` asks for; this mirrors that so
// a test can assert UserProfile leans on the server rather than filtering the page itself.
function listBlogsByOwner(...pages: Blog[][]): Api['listBlogs'] {
  const byOwner = new Map<string, Blog[][]>()
  for (const blogs of pages) {
    const ownerId = blogs[0]?.ownerId ?? ''
    byOwner.set(ownerId, [...(byOwner.get(ownerId) ?? []), blogs])
  }
  return vi.fn((params: ListBlogsParams = {}): Promise<BlogPage> => {
    const remaining = byOwner.get(params.ownerId ?? '') ?? []
    const posts = remaining.shift() ?? []
    return Promise.resolve({ posts, hasMore: remaining.length > 0 })
  })
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: listBlogsByOwner([mine], [theirs]),
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
    const listBlogs = listBlogsByOwner([mine])
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi({ listBlogs }) },
      route: '/user/calm-smiling-kestrel',
      path: '/user/:username',
    })

    expect(await screen.findByRole('heading', { name: 'calm-smiling-kestrel' })).toBeInTheDocument()
    expect(screen.getByText('Writes things.')).toBeInTheDocument()
    expect(await screen.findByText('Mine')).toBeInTheDocument()
    expect(screen.queryByText('Not mine')).not.toBeInTheDocument()
  })

  // The point of scoping by ownerId: the page only ever asks for one author's posts, not the
  // whole feed filtered client-side.
  it('fetches posts scoped to the profile uid, not the whole feed', async () => {
    const listBlogs = listBlogsByOwner([mine])
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi({ listBlogs }) },
      route: '/user/calm-smiling-kestrel',
      path: '/user/:username',
    })

    await screen.findByText('Mine')
    expect(listBlogs).toHaveBeenCalledWith({ ownerId: 'uid-1', limit: 10 })
  })

  // An author with no profile holds no username, so no URL reaches this page for them: a lookup
  // that misses means the name is genuinely unclaimed, not that the author is nameless.
  it('reports an unclaimed username as no such user', async () => {
    const listBlogs = listBlogsByOwner([mine])
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new Error('not found')), listBlogs })
    renderWithApp(<UserProfile />, {
      context: { api },
      route: '/user/nobody-here-at-all',
      path: '/user/:username',
    })

    expect(await screen.findByText('No such user.')).toBeInTheDocument()
    // Nothing to page through for a profile that never resolved, so posts are never fetched.
    expect(listBlogs).not.toHaveBeenCalled()
  })

  // Your own profile is where you look to see what your account has, so the subscription is shown
  // there as well as in the account panel.
  it('shows the caller their own subscription', async () => {
    renderWithApp(<UserProfile />, {
      context: {
        api: fakeApi(),
        profile: { ...profile, subscribedUntil: '2099-01-01T00:00:00Z' },
      },
      route: '/user/calm-smiling-kestrel',
      path: '/user/:username',
    })

    expect(await screen.findByText('Subscribed')).toBeInTheDocument()
    expect(screen.getByText(/Renews Jan 1, 2099/)).toBeInTheDocument()
  })

  // Who is paying is nobody else's business, and the public profile a lookup returns does not
  // carry it - so a signed-in reader looking at somebody else's page is shown their own
  // subscription nowhere near it, which is to say not at all.
  it("says nothing about a subscription on somebody else's profile", async () => {
    const other: User = { ...user, id: 'uid-2', username: 'bold-leaping-lynx' }
    renderWithApp(<UserProfile />, {
      context: {
        api: fakeApi({ getUser: vi.fn().mockResolvedValue(other) }),
        profile: { ...profile, subscribedUntil: '2099-01-01T00:00:00Z' },
      },
      route: '/user/bold-leaping-lynx',
      path: '/user/:username',
    })

    expect(await screen.findByRole('heading', { name: 'bold-leaping-lynx' })).toBeInTheDocument()
    expect(screen.queryByText('Subscribed')).not.toBeInTheDocument()
    expect(screen.queryByText('Not subscribed')).not.toBeInTheDocument()
  })

  // Editing is reached from the account panel alone, so the owner's own profile page carries no
  // Edit link either.
  it('leaves the Edit profile link to the account panel', async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi(), profile },
      route: '/user/calm-smiling-kestrel',
      path: '/user/:username',
    })

    expect(await screen.findByRole('heading', { name: 'calm-smiling-kestrel' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Edit profile' })).not.toBeInTheDocument()
  })

  // The backend folds case when resolving a username, so a link that differs only in case has to
  // behave identically here - header and posts alike.
  it('matches the author regardless of the case in the URL', async () => {
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi(), profile },
      route: '/user/CALM-Smiling-Kestrel',
      path: '/user/:username',
    })

    expect(await screen.findByRole('heading', { name: 'calm-smiling-kestrel' })).toBeInTheDocument()
    expect(await screen.findByText('Mine')).toBeInTheDocument()
    expect(screen.queryByText('Not mine')).not.toBeInTheDocument()
  })

  // A profile feed pages the same way the landing feed does.
  it('loads and appends the next page on "Load more"', async () => {
    const older: Blog = { ...mine, slug: 'older', title: 'Older', createdAt: '2026-01-01T00:00:00Z' }
    const listBlogs = listBlogsByOwner([mine], [older])
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi({ listBlogs }) },
      route: '/user/calm-smiling-kestrel',
      path: '/user/:username',
    })

    const loadMore = await screen.findByRole('button', { name: /Load more/ })
    await userEvent.click(loadMore)

    expect(await screen.findByText('Older')).toBeInTheDocument()
    expect(screen.getByText('Mine')).toBeInTheDocument()
    expect(listBlogs).toHaveBeenLastCalledWith({ ownerId: 'uid-1', limit: 10, startAfter: mine.createdAt })
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /Load more/ })).not.toBeInTheDocument(),
    )
  })
})
