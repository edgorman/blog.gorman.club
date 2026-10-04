import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError, type Api, type Blog, type BlogPage, type CurrentUser, type ListBlogsParams, type User } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { UserProfile } from './UserProfile'

const user: User = {
  username: 'calm-smiling-kestrel',
  bio: 'Writes things.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}
// The caller's own profile carries what their account may do as well as who they are; a public
// profile (`user` above) deliberately does not.
const profile: CurrentUser = { ...user, id: 'uid-1', assistantEnabled: false }
const mine: Blog = {
  slug: 'mine',
  authorUsername: 'calm-smiling-kestrel',
  title: 'Mine',
  content: 'hello',
  tags: [],
  visibility: 'public',
  allowedUserIds: [],
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
}
const theirs: Blog = {
  ...mine,
  slug: 'not-mine',
  authorUsername: 'bold-leaping-lynx',
  title: 'Not mine',
}

// The real backend does the author-scoping `listBlogs({ author })` asks for; this mirrors that so
// a test can assert UserProfile leans on the server rather than filtering the page itself.
function listBlogsByOwner(...pages: Blog[][]): Api['listBlogs'] {
  const byOwner = new Map<string, Blog[][]>()
  for (const blogs of pages) {
    const author = blogs[0]?.authorUsername ?? ''
    byOwner.set(author, [...(byOwner.get(author) ?? []), blogs])
  }
  return jest.fn((params: ListBlogsParams = {}): Promise<BlogPage> => {
    const remaining = byOwner.get(params.author ?? '') ?? []
    const posts = remaining.shift() ?? []
    return Promise.resolve({ posts, hasMore: remaining.length > 0 })
  })
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: listBlogsByOwner([mine], [theirs]),
    getBlog: jest.fn(),
    createBlog: jest.fn(),
    updateBlog: jest.fn(),
    deleteBlog: jest.fn(),
    getUser: jest.fn().mockResolvedValue(user),
    getCurrentUser: jest.fn(),
    putUser: jest.fn(),
    deleteUser: jest.fn(),
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

  // The point of scoping by author: the page only ever asks for one author's posts, not the
  // whole feed filtered client-side.
  it('fetches posts scoped to the profile, not the whole feed', async () => {
    const listBlogs = listBlogsByOwner([mine])
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi({ listBlogs }) },
      route: '/user/calm-smiling-kestrel',
      path: '/user/:username',
    })

    await screen.findByText('Mine')
    expect(listBlogs).toHaveBeenCalledWith({ author: 'calm-smiling-kestrel', limit: 10 })
  })

  // An author with no profile holds no username, so no URL reaches this page for them: a lookup
  // that misses means the name is genuinely unclaimed, not that the author is nameless.
  it('reports an unclaimed username as no such user', async () => {
    const listBlogs = listBlogsByOwner([mine])
    const api = fakeApi({ getUser: jest.fn().mockRejectedValue(new ApiError(404, 'user not found')), listBlogs })
    renderWithApp(<UserProfile />, {
      context: { api },
      route: '/user/nobody-here-at-all',
      path: '/user/:username',
    })

    expect(await screen.findByText('No such user.')).toBeInTheDocument()
    // Nothing to page through for a profile that never resolved, so posts are never fetched.
    expect(listBlogs).not.toHaveBeenCalled()
  })

  it('offers a retry, rather than calling the name unclaimed, when the lookup fails', async () => {
    const getUser = jest.fn().mockRejectedValueOnce(new ApiError(500, 'backend down')).mockResolvedValue(user)
    renderWithApp(<UserProfile />, {
      context: { api: fakeApi({ getUser }) },
      route: '/user/calm-smiling-kestrel',
      path: '/user/:username',
    })

    expect(await screen.findByRole('alert')).toHaveTextContent('backend down')
    expect(screen.queryByText('No such user.')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { name: user.username })).toBeInTheDocument()
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
    expect(listBlogs).toHaveBeenLastCalledWith({ author: 'calm-smiling-kestrel', limit: 10, startAfter: mine.createdAt })
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /Load more/ })).not.toBeInTheDocument(),
    )
  })
})
