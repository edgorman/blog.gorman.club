import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useApp } from '../context/AppContext'
import { errorMessage, type ChatMessage } from '../lib/api'

interface Props {
  /** The post being discussed. A chat has no identity apart from the post it is about. */
  slug: string
  /** The draft on screen, sent with every message so the assistant works on what the author sees. */
  title: string
  content: string
  /**
   * Called when the assistant edited the post. The editor has to adopt what was stored, or the
   * next save would write the pre-assistant text back over it.
   */
  onEdited: (draft: { title: string; content: string }) => void
}

/**
 * The conversation with the writing assistant, alongside the editor.
 *
 * It sends the unsaved draft with every message rather than the saved post, so "tighten this
 * paragraph" means the paragraph on screen. The assistant edits the post server-side through its
 * own tools, so a reply can change what the editor is showing - which is what `onEdited` is for.
 */
export function AssistantPanel({ slug, title, content, onEdited }: Props) {
  const { api } = useApp()
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loaded, setLoaded] = useState(false)
  const transcript = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!api) return
    let cancelled = false
    api
      .getChat(slug)
      .then((chat) => {
        if (!cancelled) setMessages(chat.messages)
      })
      // A conversation that could not be loaded is left empty rather than blocking the panel: the
      // author can still start a new one, and the backend keeps whatever is already stored.
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoaded(true)
      })
    return () => {
      cancelled = true
    }
  }, [api, slug])

  // Follows the conversation as it grows, so the newest turn is the one in view.
  useEffect(() => {
    const element = transcript.current
    if (element) element.scrollTop = element.scrollHeight
  }, [messages, sending])

  const send = () => {
    const asked = message.trim()
    if (!api || !asked || sending) return

    setSending(true)
    setError(null)
    api
      .sendChatMessage(slug, { message: asked, title, content })
      .then((reply) => {
        setMessages((previous) => [...previous, ...reply.messages])
        setMessage('')
        if (reply.updated) onEdited({ title: reply.blog.title, content: reply.blog.content })
      })
      // The message is left in the box on failure: nothing was stored, so it is still theirs to send.
      .catch((e: unknown) => setError(errorMessage(e, 'The assistant could not be reached')))
      .finally(() => setSending(false))
  }

  const clear = () => {
    if (!api || sending) return
    if (!window.confirm('Clear this conversation? Your post is not changed.')) return
    setError(null)
    api
      .clearChat(slug)
      .then(() => setMessages([]))
      .catch((e: unknown) => setError(errorMessage(e, 'Failed to clear the conversation')))
  }

  // Enter sends and shift+enter makes a new line, which is what a chat box is expected to do.
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      send()
    }
  }

  return (
    <section className="assistant" aria-label="Writing assistant">
      <header className="assistant-header">
        <h2 className="assistant-title">Assistant</h2>
        {messages.length > 0 && (
          <button type="button" className="btn btn-ghost" onClick={clear} disabled={sending}>
            Clear
          </button>
        )}
      </header>

      <div className="assistant-transcript" ref={transcript}>
        {loaded && messages.length === 0 && (
          <p className="text-muted assistant-empty">
            Ask for a rewrite, a better title, or a second opinion. Edits are applied to your post
            as you go.
          </p>
        )}

        {messages.map((turn, index) => (
          <div key={`${turn.createdAt}-${index}`} className={`assistant-turn assistant-turn-${turn.role}`}>
            {turn.content && <p className="assistant-text">{turn.content}</p>}
            {turn.edits?.map((edit, editIndex) => (
              <p key={editIndex} className="assistant-edit">
                {edit.summary}
              </p>
            ))}
          </div>
        ))}

        {sending && <p className="text-muted assistant-thinking">Thinking…</p>}
      </div>

      {error && (
        <p role="alert" className="assistant-error">
          {error}
        </p>
      )}

      <div className="assistant-compose">
        <label htmlFor="gc-assistant-message" className="sr-only">
          Message the assistant
        </label>
        <textarea
          id="gc-assistant-message"
          className="assistant-input"
          placeholder="Ask for a change…"
          value={message}
          rows={2}
          onChange={(e) => setMessage(e.target.value)}
          onKeyDown={onKeyDown}
        />
        <button
          type="button"
          className="btn btn-primary"
          onClick={send}
          disabled={sending || message.trim() === ''}
        >
          {sending ? 'Sending…' : 'Send'}
        </button>
      </div>
    </section>
  )
}
