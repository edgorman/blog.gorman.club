import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError, type Api, type Blog, type CurrentUser } from '../lib/api'
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

const profile: CurrentUser = {
  id: 'uid-1',
  username: 'edgorman',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  assistantEnabled: true,
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
    getChat: vi.fn().mockResolvedValue({ messages: [] }),
    sendChatMessage: vi.fn(),
    clearChat: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('EditPost', () => {
  it('pre-fills the editor with the existing post for its owner', async () => {
    renderWithApp(<EditPost />, {
      context: { api: fakeApi(), user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    expect(await screen.findByDisplayValue('Hello world')).toBeInTheDocument()
  })

  it('saves edits via updateBlog', async () => {
    const api = fakeApi()
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    const titleInput = await screen.findByDisplayValue('Hello world')
    await userEvent.clear(titleInput)
    await userEvent.type(titleInput, 'Updated title')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'hello-world',
      expect.objectContaining({ title: 'Updated title', visibility: 'public' }),
    )
  })

  it('carries the visibility and allowed-user whitelist through on save', async () => {
    const privateBlog: Blog = { ...blog, visibility: 'private', allowedUserIds: ['u2', 'u3'] }
    const api = fakeApi({ getBlog: vi.fn().mockResolvedValue(privateBlog) })
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'hello-world',
      expect.objectContaining({ visibility: 'private', allowedUserIds: ['u2', 'u3'] }),
    )
  })

  // The field is seeded from the post's stored tags, so an author edits what is there rather than
  // retyping the list to keep it.
  it('pre-fills and saves the tags field', async () => {
    const tagged: Blog = { ...blog, tags: ['go', 'web-dev'] }
    const api = fakeApi({ getBlog: vi.fn().mockResolvedValue(tagged) })
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    const tagsInput = await screen.findByLabelText('Tags')
    expect(tagsInput).toHaveValue('go, web-dev')

    await userEvent.clear(tagsInput)
    await userEvent.type(tagsInput, 'Rust, systems')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'hello-world',
      expect.objectContaining({ tags: ['Rust', 'systems'] }),
    )
  })

  // A blog request is a full replace, so emptying the field is how a post is untagged.
  it('clears a post\'s tags when the field is emptied', async () => {
    const tagged: Blog = { ...blog, tags: ['go'] }
    const api = fakeApi({ getBlog: vi.fn().mockResolvedValue(tagged) })
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    await userEvent.clear(await screen.findByLabelText('Tags'))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith('hello-world', expect.objectContaining({ tags: [] }))
  })

  it('switches visibility to private before saving', async () => {
    const api = fakeApi()
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('radio', { name: 'Private' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updateBlog).toHaveBeenCalledWith(
      'hello-world',
      expect.objectContaining({ visibility: 'private' }),
    )
  })

  it('refuses to edit a post owned by someone else', async () => {
    renderWithApp(<EditPost />, {
      context: { api: fakeApi(), user: stranger },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
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
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    expect(await screen.findByText('Post not found.')).toBeInTheDocument()
  })

  it('deletes the post via deleteBlog once the owner confirms', async () => {
    const api = fakeApi()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(api.deleteBlog).toHaveBeenCalledWith('hello-world')
  })

  it('does not delete the post when the owner declines the confirmation', async () => {
    const api = fakeApi()
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderWithApp(<EditPost />, {
      context: { api, user: owner },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
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
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    await screen.findByDisplayValue('Hello world')
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(await screen.findByText('nope')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Hello world')).toBeInTheDocument()
  })

  // The routes enforce the allowlist either way; rendering the panel only for an account the
  // backend would let use it is what keeps a button off the screen for somebody who would be told
  // no.
  it('offers the assistant to an account it is enabled for', async () => {
    renderWithApp(<EditPost />, {
      context: { api: fakeApi(), user: owner, profile },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    expect(await screen.findByRole('region', { name: 'Writing assistant' })).toBeInTheDocument()
  })

  it('hides the assistant from an account it is not enabled for', async () => {
    renderWithApp(<EditPost />, {
      context: { api: fakeApi(), user: owner, profile: { ...profile, assistantEnabled: false } },
      route: '/post/hello-world/edit',
      path: '/post/:slug/edit',
    })

    expect(await screen.findByDisplayValue('Hello world')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Writing assistant' })).not.toBeInTheDocument()
  })
})
