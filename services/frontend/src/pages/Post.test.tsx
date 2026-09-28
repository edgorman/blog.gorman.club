import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom'
import { AppContext } from '../context/AppContext'
import { ApiError, type Api, type Blog } from '../lib/api'
import { fakeAppContext, renderWithApp } from '../testUtils'
import { Post } from './Post'

const blog: Blog = {
  slug: 'hello-world',
  ownerId: 'uid-1',
  authorUsername: 'calm-smiling-kestrel',
  title: 'Hello world',
  content: '# Hi\n\nBody text.',
  tags: [],
  visibility: 'public',
  allowedUserIds: [],
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: jest.fn(),
    getBlog: jest.fn().mockResolvedValue(blog),
    getRelatedBlogs: jest.fn().mockResolvedValue([]),
    createBlog: jest.fn(),
    updateBlog: jest.fn(),
    deleteBlog: jest.fn(),
    getUser: jest.fn(),
    putUser: jest.fn(),
    deleteUser: jest.fn(),
    createCheckout: jest.fn(),
    createPortal: jest.fn(),
    listComments: jest.fn().mockResolvedValue([]),
    getReactions: jest.fn().mockResolvedValue({ post: [], comments: {} }),
    addReaction: jest.fn(),
    removeReaction: jest.fn(),
    createComment: jest.fn(),
    deleteComment: jest.fn(),
    ...overrides,
  } as unknown as Api
}

describe('Post', () => {
  // What search engines and link previews read when they run the page's JavaScript (#221): each
  // post's own title, description and canonical link, swapped as the reader moves between posts.
  it('updates the document title, description and canonical link when navigating between posts', async () => {
    const second: Blog = { ...blog, slug: 'second-post', title: 'Second post', content: 'Another **bold** body.' }
    const api = fakeApi({ getBlog: jest.fn((slug: string) => Promise.resolve(slug === second.slug ? second : blog)) })
    render(
      <MemoryRouter initialEntries={['/post/hello-world']}>
        <AppContext.Provider value={fakeAppContext({ api })}>
          <Link to="/post/second-post">next</Link>
          <Routes>
            <Route path="/post/:slug" element={<Post />} />
          </Routes>
        </AppContext.Provider>
      </MemoryRouter>,
    )
    const description = () => document.querySelector('meta[name="description"]')?.getAttribute('content')
    const canonical = () => document.querySelector('link[rel="canonical"]')?.getAttribute('href')

    await waitFor(() => expect(document.title).toBe('Hello world · Gorman Club'))
    expect(description()).toBe('Hi Body text.')
    expect(canonical()).toBe('https://blog.gorman.club/post/hello-world')

    await userEvent.click(screen.getByRole('link', { name: 'next' }))
    await waitFor(() => expect(document.title).toBe('Second post · Gorman Club'))
    expect(description()).toBe('Another bold body.')
    expect(canonical()).toBe('https://blog.gorman.club/post/second-post')
  })

  it('renders the fetched post, with its markdown rendered to HTML', async () => {
    renderWithApp(<Post />, { context: { api: fakeApi() }, route: '/post/hello-world', path: '/post/:slug' })

    expect(await screen.findByText('Hello world')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'Hi' })).toBeInTheDocument()
    expect(screen.getByText('Body text.')).toBeInTheDocument()
  })

  // A tag on the post page is the way into the rest of what an author wrote on that topic, so it
  // is a link to the filtered feed rather than a label.
  it('links each of the post\'s tags to the feed filtered by it', async () => {
    const api = fakeApi({ getBlog: jest.fn().mockResolvedValue({ ...blog, tags: ['go', 'web-dev'] }) })
    renderWithApp(<Post />, { context: { api }, route: '/post/hello-world', path: '/post/:slug' })

    expect(await screen.findByRole('link', { name: 'go' })).toHaveAttribute('href', '/?tag=go')
    expect(screen.getByRole('link', { name: 'web-dev' })).toHaveAttribute('href', '/?tag=web-dev')
  })

  it('renders no tag row for an untagged post', async () => {
    renderWithApp(<Post />, { context: { api: fakeApi() }, route: '/post/hello-world', path: '/post/:slug' })

    await screen.findByText('Hello world')
    expect(screen.queryByRole('list', { name: 'Tags' })).not.toBeInTheDocument()
  })

  it('lists related posts at the foot of the post, each linking to it', async () => {
    const neighbour: Blog = { ...blog, slug: 'goodbye-world', title: 'Goodbye world', content: 'Farewell.' }
    const api = fakeApi({ getRelatedBlogs: jest.fn().mockResolvedValue([neighbour]) })
    renderWithApp(<Post />, { context: { api }, route: '/post/hello-world', path: '/post/:slug' })

    expect(await screen.findByRole('heading', { level: 2, name: 'Related posts' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Goodbye world/ })).toHaveAttribute('href', '/post/goodbye-world')
    expect(api.getRelatedBlogs).toHaveBeenCalledWith('hello-world')
  })

  // No embedding yet (or a failed lookup) is not the page's problem: the section is just absent.
  it.each([
    ['no related posts', jest.fn().mockResolvedValue([])],
    ['a failed lookup', jest.fn().mockRejectedValue(new Error('boom'))],
  ])('hides the related section for %s', async (_, getRelatedBlogs) => {
    const api = fakeApi({ getRelatedBlogs })
    renderWithApp(<Post />, { context: { api }, route: '/post/hello-world', path: '/post/:slug' })

    await screen.findByText('Hello world')
    await waitFor(() => expect(getRelatedBlogs).toHaveBeenCalled())
    expect(screen.queryByRole('heading', { name: 'Related posts' })).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows a not-found message for a missing post', async () => {
    const api = fakeApi({ getBlog: jest.fn().mockRejectedValue(new ApiError(404, 'not found')) })
    renderWithApp(<Post />, { context: { api }, route: '/post/missing', path: '/post/:slug' })

    expect(await screen.findByText('Post not found.')).toBeInTheDocument()
  })

  // The backend masks a private post the caller cannot read as a 404 rather than a 403, so there
  // is nothing here to distinguish from an outright missing post - this locks that in rather than
  // reintroducing a "this post is private" state the API never triggers.
  it('treats a masked private post the same as a missing one', async () => {
    const api = fakeApi({ getBlog: jest.fn().mockRejectedValue(new ApiError(404, 'blog not found')) })
    renderWithApp(<Post />, { context: { api }, route: '/post/hello-world', path: '/post/:slug' })

    expect(await screen.findByText('Post not found.')).toBeInTheDocument()
  })

  it('scrolls to a legacy `<a name>` anchor when no element has a matching id', async () => {
    const namedAnchorBlog: Blog = {
      ...blog,
      content: '# Hi\n\n<a name="section"></a>\n\n## Section\n\nBody.',
    }
    const scrollIntoView = jest.fn()
    Element.prototype.scrollIntoView = scrollIntoView
    window.location.hash = '#section'

    renderWithApp(<Post />, {
      context: { api: fakeApi({ getBlog: jest.fn().mockResolvedValue(namedAnchorBlog) }) },
      route: '/post/hello-world#section',
      path: '/post/:slug',
    })

    await screen.findByText('Body.')
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled())
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
    const listComments = jest.fn().mockResolvedValue([
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
