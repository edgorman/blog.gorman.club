import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import type { Reactions } from '../hooks/useReactions'
import { errorMessage, userPath, type Comment } from '../lib/api'
import { formatDate } from '../lib/format'
import { ReactionBar } from './ReactionBar'

interface Props {
  /** The post being commented on. A comment has no identity apart from its post. */
  slug: string
  /**
   * The post's owner, who may delete any comment on it. The backend decides either way (see
   * `Comment.CanBeDeletedBy`); this only keeps a button off the screen for somebody who would be
   * told no.
   */
  ownerId: string
  /**
   * The page's reactions, loaded once by the post above rather than per comment: a comment's
   * reactions come back with the post's in one response, so fetching them here would be asking
   * for what the page already holds.
   */
  reactions: Reactions
}

/**
 * The comment thread beneath a post.
 *
 * A body is rendered as text rather than as markdown, unlike the post above it: a post is written
 * by the author whose page it is, while a comment is written by whoever happened to read it, and
 * the safe rendering of a stranger's input is the one that has no syntax in it at all. Line breaks
 * are kept (see `.comment-body`), which is the whole of what a comment needs.
 */
export function Comments({ slug, ownerId, reactions }: Props) {
  const { api, user } = useApp()
  const [comments, setComments] = useState<Comment[]>([])
  const [body, setBody] = useState('')
  const [sending, setSending] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!api) return
    let cancelled = false
    setLoaded(false)
    api
      .listComments(slug)
      .then((thread) => {
        if (!cancelled) setComments(thread)
      })
      // A thread that could not be loaded leaves the post readable and the box usable, rather than
      // taking the page down with it - what is stored is unaffected either way.
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e, 'Failed to load comments'))
      })
      .finally(() => {
        if (!cancelled) setLoaded(true)
      })
    return () => {
      cancelled = true
    }
  }, [api, slug])

  const send = () => {
    const said = body.trim()
    if (!api || !said || sending) return

    setSending(true)
    setError(null)
    api
      .createComment(slug, said)
      .then((created) => {
        setComments((thread) => [...thread, created])
        setBody('')
      })
      // The comment is left in the box on failure: nothing was stored, so it is still theirs to post.
      .catch((e: unknown) => setError(errorMessage(e, 'Failed to post your comment')))
      .finally(() => setSending(false))
  }

  const remove = (comment: Comment) => {
    if (!api) return
    if (!window.confirm('Delete this comment? This cannot be undone.')) return

    setError(null)
    api
      .deleteComment(slug, comment.id)
      .then(() => setComments((thread) => thread.filter((each) => each.id !== comment.id)))
      .catch((e: unknown) => setError(errorMessage(e, 'Failed to delete the comment')))
  }

  return (
    <section className="comments" aria-label="Comments">
      <h2 className="comments-title">
        {comments.length > 0 ? `${comments.length} comment${comments.length === 1 ? '' : 's'}` : 'Comments'}
      </h2>

      {loaded && comments.length === 0 && (
        <p className="text-muted comments-empty">No comments yet.</p>
      )}

      <ol className="comments-list">
        {comments.map((comment) => {
          // A comment by an author who holds no profile has no page to link to, exactly as a post
          // by one does not.
          const name = comment.authorUsername || 'an unnamed reader'
          const href = userPath(comment.authorUsername)
          const deletable = !!user && (user.id === comment.authorId || user.id === ownerId)

          return (
            <li key={comment.id} className="comment">
              <div className="comment-meta">
                {href ? (
                  <Link to={href} className="comment-author">
                    {name}
                  </Link>
                ) : (
                  <span className="comment-author">{name}</span>
                )}
                <span className="text-muted comment-date">{formatDate(comment.createdAt)}</span>
                {deletable && (
                  <button
                    type="button"
                    className="btn btn-ghost comment-delete"
                    onClick={() => remove(comment)}
                  >
                    Delete
                  </button>
                )}
              </div>
              <p className="comment-body">{comment.body}</p>
              <ReactionBar
                counts={reactions.countsFor(comment.id)}
                onToggle={(emoji) => reactions.toggle(emoji, comment.id)}
                canReact={!!user}
                label="comment"
              />
            </li>
          )
        })}
      </ol>

      {error && (
        <p role="alert" className="comments-error">
          {error}
        </p>
      )}

      {user ? (
        <div className="comments-compose">
          <label htmlFor="gc-comment-body" className="sr-only">
            Leave a comment
          </label>
          <textarea
            id="gc-comment-body"
            className="comments-input"
            placeholder="Leave a comment…"
            value={body}
            rows={3}
            onChange={(e) => setBody(e.target.value)}
          />
          <button
            type="button"
            className="btn btn-primary"
            onClick={send}
            disabled={sending || body.trim() === ''}
          >
            {sending ? 'Posting…' : 'Comment'}
          </button>
        </div>
      ) : (
        // Reading a thread never needed a credential; writing to one always does, since a comment
        // is signed by whoever left it.
        <p className="text-muted comments-signed-out">Sign in to leave a comment.</p>
      )}
    </section>
  )
}
