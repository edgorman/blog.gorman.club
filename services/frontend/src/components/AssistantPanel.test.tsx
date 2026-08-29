import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Api, Blog, ChatReply } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { AssistantPanel } from './AssistantPanel'

const blog: Blog = {
  slug: 'hello-world',
  ownerId: 'uid-1',
  authorUsername: 'calm-smiling-kestrel',
  title: 'Hello world',
  content: 'the cat sat',
  visibility: 'public',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    getChat: vi.fn().mockResolvedValue({ messages: [] }),
    sendChatMessage: vi.fn(),
    clearChat: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  } as unknown as Api
}

function reply(overrides: Partial<ChatReply> = {}): ChatReply {
  return {
    messages: [
      { role: 'user', content: 'say dog instead', createdAt: '2026-08-01T00:00:01Z' },
      {
        role: 'assistant',
        content: 'Swapped the cat for a dog.',
        edits: [{ tool: 'replace_text', summary: 'Replaced "cat" with "dog"' }],
        createdAt: '2026-08-01T00:00:02Z',
      },
    ],
    blog: { ...blog, content: 'the dog sat' },
    updated: true,
    ...overrides,
  }
}

function renderPanel(api: Api, onEdited = vi.fn()) {
  renderWithApp(
    <AssistantPanel slug="hello-world" title="Hello world" content="the cat sat" onEdited={onEdited} />,
    { context: { api } },
  )
  return onEdited
}

describe('AssistantPanel', () => {
  it('shows the stored conversation when it opens', async () => {
    const api = fakeApi({
      getChat: vi.fn().mockResolvedValue({
        messages: [
          { role: 'user', content: 'add an intro', createdAt: '2026-08-01T00:00:01Z' },
          {
            role: 'assistant',
            content: 'Added one.',
            edits: [{ tool: 'set_content', summary: 'Rewrote the post' }],
            createdAt: '2026-08-01T00:00:02Z',
          },
        ],
      }),
    })
    renderPanel(api)

    expect(await screen.findByText('add an intro')).toBeInTheDocument()
    expect(screen.getByText('Added one.')).toBeInTheDocument()
    // What it changed is shown alongside what it said, so the transcript records both.
    expect(screen.getByText('Rewrote the post')).toBeInTheDocument()
  })

  // The editor is a form with unsaved changes in it, so a request has to carry the draft on screen
  // rather than leaving the assistant to work from what was last saved.
  it('sends the draft on screen with the message', async () => {
    const sendChatMessage = vi.fn().mockResolvedValue(reply())
    renderPanel(fakeApi({ sendChatMessage }))

    await userEvent.type(screen.getByLabelText('Message the assistant'), 'say dog instead')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    await waitFor(() =>
      expect(sendChatMessage).toHaveBeenCalledWith('hello-world', {
        message: 'say dog instead',
        title: 'Hello world',
        content: 'the cat sat',
      }),
    )
  })

  // The assistant edits the post server-side, so the editor has to adopt what was stored - or the
  // next save would write the pre-assistant text back over it.
  it('hands the edited post back to the editor', async () => {
    const onEdited = renderPanel(fakeApi({ sendChatMessage: vi.fn().mockResolvedValue(reply()) }))

    await userEvent.type(screen.getByLabelText('Message the assistant'), 'say dog instead')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    await waitFor(() =>
      expect(onEdited).toHaveBeenCalledWith({ title: 'Hello world', content: 'the dog sat' }),
    )
    expect(await screen.findByText('Swapped the cat for a dog.')).toBeInTheDocument()
  })

  // A turn that only answered a question must not disturb what the author is typing.
  it('leaves the editor alone when nothing was edited', async () => {
    const answer = reply({
      messages: [
        { role: 'user', content: 'is it ok?', createdAt: '2026-08-01T00:00:01Z' },
        { role: 'assistant', content: 'It reads well.', createdAt: '2026-08-01T00:00:02Z' },
      ],
      blog,
      updated: false,
    })
    const onEdited = renderPanel(fakeApi({ sendChatMessage: vi.fn().mockResolvedValue(answer) }))

    await userEvent.type(screen.getByLabelText('Message the assistant'), 'is it ok?')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    expect(await screen.findByText('It reads well.')).toBeInTheDocument()
    expect(onEdited).not.toHaveBeenCalled()
  })

  // Nothing was stored for a failed turn, so the message is still the author's to send again.
  it('reports a failure and keeps the message in the box', async () => {
    const sendChatMessage = vi.fn().mockRejectedValue(new Error('the assistant could not be reached'))
    renderPanel(fakeApi({ sendChatMessage }))

    const box = screen.getByLabelText('Message the assistant')
    await userEvent.type(box, 'rewrite it')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('the assistant could not be reached')
    expect(box).toHaveValue('rewrite it')
  })

  it('will not send an empty message', async () => {
    const sendChatMessage = vi.fn()
    renderPanel(fakeApi({ sendChatMessage }))

    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()

    await userEvent.type(screen.getByLabelText('Message the assistant'), '   ')

    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    expect(sendChatMessage).not.toHaveBeenCalled()
  })

  // Starting the assistant over must not start the post over.
  it('clears the conversation without touching the post', async () => {
    const clearChat = vi.fn().mockResolvedValue(undefined)
    const api = fakeApi({
      getChat: vi
        .fn()
        .mockResolvedValue({ messages: [{ role: 'user', content: 'add an intro', createdAt: '2026-08-01T00:00:01Z' }] }),
      clearChat,
    })
    const onEdited = renderPanel(api)
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    await userEvent.click(await screen.findByRole('button', { name: 'Clear' }))

    await waitFor(() => expect(clearChat).toHaveBeenCalledWith('hello-world'))
    expect(screen.queryByText('add an intro')).not.toBeInTheDocument()
    expect(onEdited).not.toHaveBeenCalled()
  })
})
