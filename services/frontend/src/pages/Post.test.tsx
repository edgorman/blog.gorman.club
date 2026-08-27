import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ApiError, type Api, type Blog } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { Post } from './Post'

const blog: Blog = {
  id: 'p1',
  ownerId: 'uid-1',
  title: 'Hello world',
  content: '# Hi\n\nBody text.',
  visibility: 'public',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: vi.fn(),
    getBlog: vi.fn().mockResolvedValue(blog),
    createBlog: vi.fn(),
    updateBlog: vi.fn(),
    deleteBlog: vi.fn(),
    getUser: vi.fn(),
    putUser: vi.fn(),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('Post', () => {
  it('renders the fetched post, with its markdown rendered to HTML', async () => {
    renderWithApp(<Post />, { context: { api: fakeApi() }, route: '/post/p1', path: '/post/:id' })

    expect(await screen.findByText('Hello world')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'Hi' })).toBeInTheDocument()
    expect(screen.getByText('Body text.')).toBeInTheDocument()
  })

  it('shows a not-found message for a missing post', async () => {
    const api = fakeApi({ getBlog: vi.fn().mockRejectedValue(new ApiError(404, 'not found')) })
    renderWithApp(<Post />, { context: { api }, route: '/post/missing', path: '/post/:id' })

    expect(await screen.findByText('Post not found.')).toBeInTheDocument()
  })

  it('shows a private-post message when forbidden', async () => {
    const api = fakeApi({ getBlog: vi.fn().mockRejectedValue(new ApiError(403, 'forbidden')) })
    renderWithApp(<Post />, { context: { api }, route: '/post/p1', path: '/post/:id' })

    expect(await screen.findByText('This post is private.')).toBeInTheDocument()
  })
})
