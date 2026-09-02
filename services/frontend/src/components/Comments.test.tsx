import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Reactions } from '../hooks/useReactions'
import { ApiError, type Api, type Comment } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { Comments } from './Comments'

const OWNER = 'uid-owner'
const READER = 'uid-reader'

function comment(overrides: Partial<Comment> = {}): Comment {
  return {
    id: 'cmt1',
    blogSlug: 'hello-world',
    authorId: READER,
    authorUsername: 'sly-dancing-monkey',
    body: 'Nicely put.',
    createdAt: '2026-08-02T00:00:00Z',
    ...overrides,
  }
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listComments: vi.fn().mockResolvedValue([]),
    createComment: vi.fn(),
    deleteComment: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  } as unknown as Api
}

/** The page's reactions, which the post above normally owns; these tests are about the thread. */
function noReactions(overrides: Partial<Reactions> = {}): Reactions {
  return { countsFor: () => [], toggle: vi.fn(), error: null, ...overrides }
}

/** Renders the thread as the given signed-in reader, or signed out when uid is omitted. */
function renderComments(api: Api, uid?: string, reactions: Reactions = noReactions()) {
  return renderWithApp(<Comments slug="hello-world" ownerId={OWNER} reactions={reactions} />, {
    context: { api, user: uid ? { id: uid, email: 'reader@example.com', name: 'Reader' } : null },
  })
}

describe('Comments', () => {
  it('shows the stored thread, attributed and linked to its authors', async () => {
    const api = fakeApi({ listComments: vi.fn().mockResolvedValue([comment()]) })
    renderComments(api, READER)

    expect(await screen.findByText('Nicely put.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'sly-dancing-monkey' })).toHaveAttribute(
      'href',
      '/user/sly-dancing-monkey',
    )
    expect(api.listComments).toHaveBeenCalledWith('hello-world')
  })

  it('says so when a post has no comments yet', async () => {
    renderComments(fakeApi(), READER)

    expect(await screen.findByText('No comments yet.')).toBeInTheDocument()
  })

  // A comment is signed by whoever left it, so there is nothing to send without a credential - the
  // thread itself stays readable.
  it('offers a signed-out reader no way to comment', async () => {
    const api = fakeApi({ listComments: vi.fn().mockResolvedValue([comment()]) })
    renderComments(api)

    expect(await screen.findByText('Nicely put.')).toBeInTheDocument()
    expect(screen.getByText('Sign in to leave a comment.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Comment' })).not.toBeInTheDocument()
  })

  it('posts a comment and appends it to the thread', async () => {
    const created = comment({ id: 'cmt2', body: 'Well said.' })
    const api = fakeApi({ createComment: vi.fn().mockResolvedValue(created) })
    renderComments(api, READER)

    await userEvent.type(await screen.findByLabelText('Leave a comment'), 'Well said.')
    await userEvent.click(screen.getByRole('button', { name: 'Comment' }))

    await waitFor(() => expect(api.createComment).toHaveBeenCalledWith('hello-world', 'Well said.'))
    expect(await screen.findByText('Well said.')).toBeInTheDocument()
    // Sent, so the box is cleared - unlike the failure below, where it is still theirs to post.
    expect(await screen.findByLabelText('Leave a comment')).toHaveValue('')
  })

  it('will not send an empty comment', async () => {
    const api = fakeApi()
    renderComments(api, READER)

    await userEvent.type(await screen.findByLabelText('Leave a comment'), '   ')

    expect(screen.getByRole('button', { name: 'Comment' })).toBeDisabled()
    expect(api.createComment).not.toHaveBeenCalled()
  })

  it('keeps the comment in the box when posting fails', async () => {
    const api = fakeApi({
      createComment: vi.fn().mockRejectedValue(new ApiError(500, 'internal error')),
    })
    renderComments(api, READER)

    await userEvent.type(await screen.findByLabelText('Leave a comment'), 'Well said.')
    await userEvent.click(screen.getByRole('button', { name: 'Comment' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('internal error')
    expect(screen.getByLabelText('Leave a comment')).toHaveValue('Well said.')
  })

  // Who may delete is the backend's decision either way; this is about which button is offered.
  it('offers Delete to a comment’s author and to the post’s owner, and to nobody else', async () => {
    const api = () => fakeApi({ listComments: vi.fn().mockResolvedValue([comment()]) })

    const asAuthor = renderComments(api(), READER)
    expect(await screen.findByRole('button', { name: 'Delete' })).toBeInTheDocument()
    asAuthor.unmount()

    const asOwner = renderComments(api(), OWNER)
    expect(await screen.findByRole('button', { name: 'Delete' })).toBeInTheDocument()
    asOwner.unmount()

    const asStranger = renderComments(api(), 'uid-stranger')
    expect(await screen.findByText('Nicely put.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
    asStranger.unmount()

    renderComments(api())
    expect(await screen.findByText('Nicely put.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
  })

  it('deletes a comment once it is confirmed, and drops it from the thread', async () => {
    const api = fakeApi({ listComments: vi.fn().mockResolvedValue([comment()]) })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderComments(api, OWNER)

    await userEvent.click(await screen.findByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(api.deleteComment).toHaveBeenCalledWith('hello-world', 'cmt1'))
    await waitFor(() => expect(screen.queryByText('Nicely put.')).not.toBeInTheDocument())
    vi.restoreAllMocks()
  })

  it('leaves the comment alone when the confirmation is dismissed', async () => {
    const api = fakeApi({ listComments: vi.fn().mockResolvedValue([comment()]) })
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderComments(api, OWNER)

    await userEvent.click(await screen.findByRole('button', { name: 'Delete' }))

    expect(api.deleteComment).not.toHaveBeenCalled()
    expect(screen.getByText('Nicely put.')).toBeInTheDocument()
    vi.restoreAllMocks()
  })

  // A comment is written by whoever happened to read the post, so its body is rendered as text: no
  // markdown, and nothing that could be mistaken for markup.
  it('renders a comment body as text rather than as markup', async () => {
    const api = fakeApi({
      listComments: vi
        .fn()
        .mockResolvedValue([comment({ body: '# not a heading <img src=x onerror=alert(1)>' })]),
    })
    const { container } = renderComments(api, READER)

    expect(await screen.findByText('# not a heading <img src=x onerror=alert(1)>')).toBeInTheDocument()
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('h1')).toBeNull()
  })

  // A thread that could not be loaded leaves the post readable and the box usable.
  it('reports a thread it could not load without hiding the compose box', async () => {
    const api = fakeApi({
      listComments: vi.fn().mockRejectedValue(new ApiError(500, 'internal error')),
    })
    renderComments(api, READER)

    expect(await screen.findByRole('alert')).toHaveTextContent('internal error')
    expect(screen.getByRole('button', { name: 'Comment' })).toBeInTheDocument()
  })
})
