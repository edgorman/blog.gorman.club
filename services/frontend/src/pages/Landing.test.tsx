import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Api, Blog, BlogPage } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { Landing } from './Landing'

function post(overrides: Partial<Blog>): Blog {
  return {
    slug: 'hello-world',
    ownerId: 'uid-1',
    authorUsername: 'calm-smiling-kestrel',
    title: 'Untitled',
    content: 'hello',
    visibility: 'public',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
    ...overrides,
  }
}

function page(posts: Blog[], hasMore = false): BlogPage {
  return { posts, hasMore }
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: vi.fn().mockResolvedValue(page([])),
    getBlog: vi.fn(),
    createBlog: vi.fn(),
    updateBlog: vi.fn(),
    deleteBlog: vi.fn(),
    getUser: vi.fn(),
    putUser: vi.fn(),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('Landing', () => {
  it('shows a message when no backend is configured', () => {
    renderWithApp(<Landing />, { context: { api: null } })
    expect(screen.getByText(/No backend deployed yet/)).toBeInTheDocument()
  })

  // The backend answers newest first; Landing renders the page as it arrives rather than
  // re-sorting it.
  it('renders posts in the order the backend returns them', async () => {
    const posts = [
      post({ slug: 'new-post', title: 'New post', createdAt: '2026-08-20T00:00:00Z' }),
      post({ slug: 'old-post', title: 'Old post', createdAt: '2026-01-01T00:00:00Z' }),
    ]
    const api = fakeApi({ listBlogs: vi.fn().mockResolvedValue(page(posts)) })
    renderWithApp(<Landing />, { context: { api } })

    const titles = await screen.findAllByRole('heading', { level: 2 })
    expect(titles.map((t) => t.textContent)).toEqual(['New post', 'Old post'])
  })

  // A slug names one post across every author, so a row links to the slug alone.
  it('links each row to its slug', async () => {
    const posts = [post({ slug: 'hello-world', title: 'Hello world' })]
    const api = fakeApi({ listBlogs: vi.fn().mockResolvedValue(page(posts)) })
    renderWithApp(<Landing />, { context: { api } })

    const row = await screen.findByRole('link', { name: /Hello world/ })
    expect(row).toHaveAttribute('href', '/post/hello-world')
  })

  // A post's address no longer runs through its author, so one whose owner holds no username is
  // still reachable - it is only shown without an author beside it.
  it('links a row whose author has no username', async () => {
    const posts = [post({ slug: 'orphaned', title: 'Orphaned', authorUsername: '' })]
    const api = fakeApi({ listBlogs: vi.fn().mockResolvedValue(page(posts)) })
    renderWithApp(<Landing />, { context: { api } })

    expect(await screen.findByRole('link', { name: /Orphaned/ })).toHaveAttribute(
      'href',
      '/post/orphaned',
    )
  })

  it('shows an empty state when there are no posts', async () => {
    renderWithApp(<Landing />, { context: { api: fakeApi() } })
    expect(await screen.findByText(/No posts yet/)).toBeInTheDocument()
  })

  it('surfaces a load failure instead of an empty feed', async () => {
    const api = fakeApi({ listBlogs: vi.fn().mockRejectedValue(new Error('boom')) })
    renderWithApp(<Landing />, { context: { api } })
    expect(await screen.findByText('boom')).toBeInTheDocument()
  })

  // The very point of pagination: the first call asks for one page, not the whole collection.
  it('requests only the first page on load', async () => {
    const listBlogs = vi.fn().mockResolvedValue(page([post({})]))
    renderWithApp(<Landing />, { context: { api: fakeApi({ listBlogs }) } })

    await screen.findAllByRole('link')
    expect(listBlogs).toHaveBeenCalledWith({ limit: 10 })
  })

  it('offers no "Load more" once the backend reports no further page', async () => {
    const api = fakeApi({ listBlogs: vi.fn().mockResolvedValue(page([post({})], false)) })
    renderWithApp(<Landing />, { context: { api } })

    await screen.findAllByRole('link')
    expect(screen.queryByRole('button', { name: /Load more/ })).not.toBeInTheDocument()
  })

  // Clicking "Load more" continues from the last post already shown, and appends rather than
  // replaces what is on screen.
  it('loads and appends the next page on "Load more"', async () => {
    const firstPost = post({ slug: 'first', title: 'First', createdAt: '2026-08-20T00:00:00Z' })
    const secondPost = post({ slug: 'second', title: 'Second', createdAt: '2026-01-01T00:00:00Z' })
    const listBlogs = vi
      .fn()
      .mockResolvedValueOnce(page([firstPost], true))
      .mockResolvedValueOnce(page([secondPost], false))
    const api = fakeApi({ listBlogs })
    renderWithApp(<Landing />, { context: { api } })

    const loadMore = await screen.findByRole('button', { name: /Load more/ })
    await userEvent.click(loadMore)

    expect(await screen.findByText('Second')).toBeInTheDocument()
    expect(screen.getByText('First')).toBeInTheDocument()
    expect(listBlogs).toHaveBeenLastCalledWith({ limit: 10, startAfter: firstPost.createdAt })
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /Load more/ })).not.toBeInTheDocument(),
    )
  })
})
