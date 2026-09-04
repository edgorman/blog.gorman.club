import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Api, Blog } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { NewPost } from './NewPost'

const author = { id: 'uid-1', email: 'a@b.com', name: 'Ada' }

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: vi.fn(),
    getBlog: vi.fn(),
    createBlog: vi
      .fn()
      .mockResolvedValue({ slug: 'my-post', authorUsername: 'calm-smiling-kestrel', title: 'My post' } as Blog),
    updateBlog: vi.fn(),
    deleteBlog: vi.fn(),
    getUser: vi.fn(),
    putUser: vi.fn(),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('NewPost', () => {
  it('prompts sign-in when signed out', () => {
    renderWithApp(<NewPost />, { context: { api: fakeApi(), user: null } })
    expect(screen.getByText('Sign in to publish a post.')).toBeInTheDocument()
  })

  it('publishes the draft as a public post and shows a confirmation', async () => {
    const api = fakeApi()
    renderWithApp(<NewPost />, { context: { api, user: author } })

    await userEvent.type(screen.getByLabelText('Title'), 'My post')
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

    await screen.findByText('Published.')
    expect(api.createBlog).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'My post', visibility: 'public' }),
    )
    // The link is built from both halves of the address the backend assigned.
    expect(screen.getByRole('link', { name: 'View post' })).toHaveAttribute(
      'href',
      '/post/my-post',
    )
  })

  // The field is one line of text; splitting it on commas is all the client does, since the tags
  // themselves are normalized server-side.
  it('splits the tags field on commas when publishing', async () => {
    const api = fakeApi()
    renderWithApp(<NewPost />, { context: { api, user: author } })

    await userEvent.type(screen.getByLabelText('Title'), 'My post')
    await userEvent.type(screen.getByLabelText('Tags'), 'Go, web dev,')
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

    await screen.findByText('Published.')
    expect(api.createBlog).toHaveBeenCalledWith(
      expect.objectContaining({ tags: ['Go', 'web dev'] }),
    )
  })

  it('publishes an untagged post with no tags', async () => {
    const api = fakeApi()
    renderWithApp(<NewPost />, { context: { api, user: author } })

    await userEvent.type(screen.getByLabelText('Title'), 'My post')
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

    await screen.findByText('Published.')
    expect(api.createBlog).toHaveBeenCalledWith(expect.objectContaining({ tags: [] }))
  })

  it('publishes as private when that visibility is chosen', async () => {
    const api = fakeApi()
    renderWithApp(<NewPost />, { context: { api, user: author } })

    await userEvent.type(screen.getByLabelText('Title'), 'My post')
    await userEvent.click(screen.getByRole('radio', { name: 'Private' }))
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

    await screen.findByText('Published.')
    expect(api.createBlog).toHaveBeenCalledWith(expect.objectContaining({ visibility: 'private' }))
  })

  it('renders a live preview of the markdown draft', async () => {
    renderWithApp(<NewPost />, { context: { api: fakeApi(), user: author } })

    await userEvent.click(screen.getByRole('radio', { name: 'Preview' }))
    expect(screen.getByRole('heading', { name: 'Your title here' })).toBeInTheDocument()
  })
})
