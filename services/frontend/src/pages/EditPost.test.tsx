import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError, type Api, type Blog } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { EditPost } from './EditPost'

const owner = { id: 'uid-1', email: 'a@b.com', name: 'Ada' }
const stranger = { id: 'someone-else', email: 'x@y.com', name: 'Bo' }

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
    updateBlog: vi.fn().mockResolvedValue(blog),
    deleteBlog: vi.fn(),
    getUser: vi.fn(),
    putUser: vi.fn(),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('EditPost', () => {
  it('pre-fills the editor with the existing post for its owner', async () => {
    renderWithApp(<EditPost />, {
      context: { api: fakeApi(), user: owner },
      route: '/post/p1/edit',
      path: '/post/:id/edit',
    })

    expect(await screen.findByDisplayValue('Hello world')).toBeInTheDocument()
  })

  it('saves edits via updateBlog', async () => {
    const api = fakeApi()
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/p1/edit',
      path: '/post/:id/edit',
    })

    const titleInput = await screen.findByDisplayValue('Hello world')
    await userEvent.clear(titleInput)
    await userEvent.type(titleInput, 'Updated title')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'p1',
      expect.objectContaining({ title: 'Updated title', visibility: 'public' }),
    )
  })

  it('carries the visibility and allowed-user whitelist through on save', async () => {
    const privateBlog: Blog = { ...blog, visibility: 'private', allowedUserIds: ['u2', 'u3'] }
    const api = fakeApi({ getBlog: vi.fn().mockResolvedValue(privateBlog) })
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/p1/edit',
      path: '/post/:id/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'p1',
      expect.objectContaining({ visibility: 'private', allowedUserIds: ['u2', 'u3'] }),
    )
  })

  it('switches visibility to private before saving', async () => {
    const api = fakeApi()
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/p1/edit',
      path: '/post/:id/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('radio', { name: 'Private' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith('p1', expect.objectContaining({ visibility: 'private' }))
  })

  it('refuses to edit a post owned by someone else', async () => {
    renderWithApp(<EditPost />, {
      context: { api: fakeApi(), user: stranger },
      route: '/post/p1/edit',
      path: '/post/:id/edit',
    })

    expect(await screen.findByText("You don't have permission to edit this post.")).toBeInTheDocument()
  })

  it('shows a private-post message when forbidden', async () => {
    const api = fakeApi({ getBlog: vi.fn().mockRejectedValue(new ApiError(403, 'forbidden')) })
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/p1/edit',
      path: '/post/:id/edit',
    })

    expect(await screen.findByText('This post is private.')).toBeInTheDocument()
  })
})
