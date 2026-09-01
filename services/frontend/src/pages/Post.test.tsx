import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ApiError, type Api, type Blog } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { Post } from './Post'

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
    updateBlog: vi.fn(),
    deleteBlog: vi.fn(),
    getUser: vi.fn(),
    putUser: vi.fn(),
    deleteUser: vi.fn(),
    listComments: vi.fn().mockResolvedValue([]),
    getReactions: vi.fn().mockResolvedValue({ post: [], comments: {} }),
    addReaction: vi.fn(),
    removeReaction: vi.fn(),
    createComment: vi.fn(),
    deleteComment: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('Post', () => {
  it('renders the fetched post, with its markdown rendered to HTML', async () => {
    renderWithApp(<Post />, { context: { api: fakeApi() }, route: '/post/hello-world', path: '/post/:slug' })

    expect(await screen.findByText('Hello world')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'Hi' })).toBeInTheDocument()
    expect(screen.getByText('Body text.')).toBeInTheDocument()
  })

  it('shows a not-found message for a missing post', async () => {
    const api = fakeApi({ getBlog: vi.fn().mockRejectedValue(new ApiError(404, 'not found')) })
    renderWithApp(<Post />, { context: { api }, route: '/post/missing', path: '/post/:slug' })

    expect(await screen.findByText('Post not found.')).toBeInTheDocument()
  })

  // The backend masks a private post the caller cannot read as a 404 rather than a 403, so there
  // is nothing here to distinguish from an outright missing post - this locks that in rather than
  // reintroducing a "this post is private" state the API never triggers.
  it('treats a masked private post the same as a missing one', async () => {
    const api = fakeApi({ getBlog: vi.fn().mockRejectedValue(new ApiError(404, 'blog not found')) })
    renderWithApp(<Post />, { context: { api }, route: '/post/hello-world', path: '/post/:slug' })

    expect(await screen.findByText('Post not found.')).toBeInTheDocument()
  })

  it('scrolls to a legacy `<a name>` anchor when no element has a matching id', async () => {
    const namedAnchorBlog: Blog = {
      ...blog,
      content: '# Hi\n\n<a name="section"></a>\n\n## Section\n\nBody.',
    }
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView
    window.location.hash = '#section'

    renderWithApp(<Post />, {
      context: { api: fakeApi({ getBlog: vi.fn().mockResolvedValue(namedAnchorBlog) }) },
      route: '/post/hello-world#section',
      path: '/post/:slug',
    })

    await screen.findByText('Body.')
    await vi.waitFor(() => expect(scrollIntoView).toHaveBeenCalled())
    window.location.hash = ''
  })

  it('shows an Edit link only to the post owner', async () => {
    renderWithApp(<Post />, {
      context: { api: fakeApi(), user: { id: 'uid-1', email: 'a@b.com', name: 'Ada' } },
      route: '/post/hello-world',
      path: '/post/:slug',
    })
    expect(await screen.findByRole('link', { name: 'Edit' })).toHaveAttribute(
      'href',
      '/post/hello-world/edit',
    )
  })

  it('hides the Edit link from a signed-in visitor who is not the owner', async () => {
    renderWithApp(<Post />, {
      context: { api: fakeApi(), user: { id: 'someone-else', email: 'x@y.com', name: 'Bo' } },
      route: '/post/hello-world',
      path: '/post/:slug',
    })
    expect(await screen.findByText('Hello world')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Edit' })).not.toBeInTheDocument()
  })

  // The thread is the readers' half of the page, and hangs off the post that was just loaded - so
  // it is fetched by the same slug, for whoever could read the post at all.
  it('shows the comment thread beneath the post', async () => {
    const listComments = vi.fn().mockResolvedValue([
      {
        id: 'cmt1',
        blogSlug: 'hello-world',
        authorId: 'uid-2',
        authorUsername: 'sly-dancing-monkey',
        body: 'Nicely put.',
        createdAt: '2026-08-02T00:00:00Z',
      },
    ])
    renderWithApp(<Post />, {
      context: { api: fakeApi({ listComments }) },
      route: '/post/hello-world',
      path: '/post/:slug',
    })

    expect(await screen.findByText('Nicely put.')).toBeInTheDocument()
    expect(listComments).toHaveBeenCalledWith('hello-world')
  })
})
