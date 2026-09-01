import { useCallback, useEffect, useMemo, useState } from 'react'
import { useApp } from '../context/AppContext'
import { errorMessage, type PageReactions, type ReactionCount } from '../lib/api'

/**
 * The reactions on one post page: the post's own and every comment's, loaded together because the
 * API answers them together.
 *
 * `toggle` decides which way to write from what is currently stored - the API has no toggle of its
 * own, deliberately, so that a stale page or a retried click lands where it was aiming rather than
 * undoing itself. What comes back is the target's whole count, so another reader's click that
 * arrived in between is picked up rather than overwritten by an optimistic guess.
 */
export interface Reactions {
  /** The counts on one target: the post when `commentId` is omitted, or one of its comments. */
  countsFor: (commentId?: string) => ReactionCount[]
  toggle: (emoji: string, commentId?: string) => void
  error: string | null
}

const NONE: ReactionCount[] = []

export function useReactions(slug: string): Reactions {
  const { api } = useApp()
  const [reactions, setReactions] = useState<PageReactions>({ post: [], comments: {} })
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!api) return
    let cancelled = false
    api
      .getReactions(slug)
      .then((page) => {
        if (!cancelled) setReactions(page)
      })
      // A page whose reactions could not be loaded still reads: the post and its comments are the
      // point, and a bar nobody can see is not worth an error above them.
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [api, slug])

  const countsFor = useCallback(
    (commentId?: string) =>
      commentId === undefined ? reactions.post : (reactions.comments[commentId] ?? NONE),
    [reactions],
  )

  const toggle = useCallback(
    (emoji: string, commentId?: string) => {
      if (!api) return

      const mine = countsFor(commentId).find((count) => count.emoji === emoji)
      const write = mine?.reacted ? api.removeReaction : api.addReaction

      setError(null)
      write(slug, emoji, commentId)
        .then((counts) =>
          setReactions((page) =>
            commentId === undefined
              ? { ...page, post: counts }
              : { ...page, comments: { ...page.comments, [commentId]: counts } },
          ),
        )
        .catch((e: unknown) => setError(errorMessage(e, 'Failed to save your reaction')))
    },
    [api, countsFor, slug],
  )

  return useMemo(() => ({ countsFor, toggle, error }), [countsFor, toggle, error])
}
