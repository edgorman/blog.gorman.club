import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Blogs } from './Blogs'
import type { Api, Blog } from '../lib/api'

const own: Blog = {
  id: 'blog-1',
  ownerId: 'caller',
  title: 'Mine',
  content: 'hello',
  visibility: 'public',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const theirs: Blog = { ...own, id: 'blog-2', ownerId: 'someone-else', title: 'Theirs' }

function fakeApi(overrides: Partial<Api> = {}) {
  return {
    listBlogs: vi.fn().mockResolvedValue([own, theirs]),
    createBlog: vi.fn().mockResolvedValue(own),
    updateBlog: vi.fn().mockResolvedValue(own),
    deleteBlog: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  } as unknown as Api
}

describe('Blogs', () => {
  it('lists what the API returns', async () => {
    render(<Blogs api={fakeApi()} uid="caller" />)

    expect(await screen.findByText('Mine')).toBeInTheDocument()
    expect(screen.getByText('Theirs')).toBeInTheDocument()
    expect(screen.getByText('2 readable blogs')).toBeInTheDocument()
  })

  // The API 403s a write from a non-owner, so the UI must not offer one.
  it('offers Edit and Delete only for blogs the caller owns', async () => {
    render(<Blogs api={fakeApi()} uid="caller" />)

    await screen.findByText('Mine')
    expect(screen.getAllByRole('button', { name: 'Edit' })).toHaveLength(1)
    expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(1)
    expect(screen.getByText("someone else's")).toBeInTheDocument()
  })

  it('creates a blog from the draft and reloads the list', async () => {
    const api = fakeApi()
    render(<Blogs api={api} uid="caller" />)
    await screen.findByText('Mine')

    await userEvent.type(screen.getByLabelText(/Title/), 'New post')
    await userEvent.type(screen.getByLabelText(/Content/), 'body')
    await userEvent.selectOptions(screen.getByLabelText(/Visibility/), 'private')
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() =>
      expect(api.createBlog).toHaveBeenCalledWith({
        title: 'New post',
        content: 'body',
        visibility: 'private',
      }),
    )
    expect(api.listBlogs).toHaveBeenCalledTimes(2)
  })

  it('switches to update when a blog is loaded for editing', async () => {
    const api = fakeApi()
    render(<Blogs api={api} uid="caller" />)
    await screen.findByText('Mine')

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getByLabelText(/Title/)).toHaveValue('Mine')

    await userEvent.click(screen.getByRole('button', { name: 'Update' }))
    await waitFor(() => expect(api.updateBlog).toHaveBeenCalledWith('blog-1', expect.anything()))
    expect(api.createBlog).not.toHaveBeenCalled()
  })

  it('surfaces an API failure instead of rendering an empty list silently', async () => {
    const api = fakeApi({ listBlogs: vi.fn().mockRejectedValue(new Error('boom')) })
    render(<Blogs api={api} uid="caller" />)

    expect(await screen.findByText('boom')).toBeInTheDocument()
  })
})
