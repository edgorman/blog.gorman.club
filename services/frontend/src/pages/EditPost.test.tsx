import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError, type Api, type Blog } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { EditPost } from './EditPost'

const owner = { id: 'uid-1', email: 'a@b.com', name: 'Ada' }
const stranger = { id: 'someone-else', email: 'x@y.com', name: 'Bo' }

const blog: Blog = {
  slug: 'hello-world',
  ownerId: 'uid-1',
  authorUsername: 'calm-smiling-kestrel',
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
    deleteBlog: vi.fn().mockResolvedValue(undefined),
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
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    expect(await screen.findByDisplayValue('Hello world')).toBeInTheDocument()
  })

  it('saves edits via updateBlog', async () => {
    const api = fakeApi()
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    const titleInput = await screen.findByDisplayValue('Hello world')
    await userEvent.clear(titleInput)
    await userEvent.type(titleInput, 'Updated title')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'calm-smiling-kestrel',
      'hello-world',
      expect.objectContaining({ title: 'Updated title', visibility: 'public' }),
    )
  })

  it('carries the visibility and allowed-user whitelist through on save', async () => {
    const privateBlog: Blog = { ...blog, visibility: 'private', allowedUserIds: ['u2', 'u3'] }
    const api = fakeApi({ getBlog: vi.fn().mockResolvedValue(privateBlog) })
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'calm-smiling-kestrel',
      'hello-world',
      expect.objectContaining({ visibility: 'private', allowedUserIds: ['u2', 'u3'] }),
    )
  })

  it('switches visibility to private before saving', async () => {
    const api = fakeApi()
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('radio', { name: 'Private' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'calm-smiling-kestrel',
      'hello-world',
      expect.objectContaining({ visibility: 'private' }),
    )
  })

  it('refuses to edit a post owned by someone else', async () => {
    renderWithApp(<EditPost />, {
      context: { api: fakeApi(), user: stranger },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    expect(await screen.findByText("You don't have permission to edit this post.")).toBeInTheDocument()
  })

  // The backend masks a private post the caller cannot read as a 404 rather than a 403, so there
  // is nothing here to distinguish from an outright missing post - this locks that in rather than
  // reintroducing a "this post is private" state the API never triggers.
  it('treats a masked private post the same as a missing one', async () => {
    const api = fakeApi({ getBlog: vi.fn().mockRejectedValue(new ApiError(404, 'blog not found')) })
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    expect(await screen.findByText('Post not found.')).toBeInTheDocument()
  })

  it('deletes the post via deleteBlog once the owner confirms', async () => {
    const api = fakeApi()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(api.deleteBlog).toHaveBeenCalledWith('calm-smiling-kestrel', 'hello-world')
  })

  it('does not delete the post when the owner declines the confirmation', async () => {
    const api = fakeApi()
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(api.deleteBlog).not.toHaveBeenCalled()
  })

  it('shows an error and leaves the editor in place when the delete request fails', async () => {
    const api = fakeApi({ deleteBlog: vi.fn().mockRejectedValue(new Error('nope')) })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/calm-smiling-kestrel/hello-world/edit',
      path: '/post/:username/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(await screen.findByText('nope')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Hello world')).toBeInTheDocument()
  })
})
