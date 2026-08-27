import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Api, Blog } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { Landing } from './Landing'

function post(overrides: Partial<Blog>): Blog {
  return {
    id: 'p1',
    ownerId: 'uid-1',
    title: 'Untitled',
    content: 'hello',
    visibility: 'public',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
    ...overrides,
  }
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: vi.fn().mockResolvedValue([]),
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

  it('renders posts newest first', async () => {
    const posts = [
      post({ id: 'old', title: 'Old post', createdAt: '2026-01-01T00:00:00Z' }),
      post({ id: 'new', title: 'New post', createdAt: '2026-08-20T00:00:00Z' }),
    ]
    const api = fakeApi({ listBlogs: vi.fn().mockResolvedValue(posts) })
    renderWithApp(<Landing />, { context: { api } })

    const titles = await screen.findAllByRole('heading', { level: 2 })
    expect(titles.map((t) => t.textContent)).toEqual(['New post', 'Old post'])
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
})
