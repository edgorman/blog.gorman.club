import { useState } from 'react'
import type { ReactionCount } from '../lib/api'

/**
 * What the picker offers. The API accepts any emoji (see the backend's `ValidEmoji`), so this is a
 * convenience rather than the rule: a wider picker - or one that lets a reader type their own - is
 * a change to this array and nothing else.
 */
const PICKER = [
  '👍', '👎', '❤️', '🎉', '😂', '😮',
  '😢', '🔥', '💯', '🙏', '👀', '🤔',
  '🚀', '✅', '⭐', '💡', '🧠', '☕',
  '🐛', '📚', '🥳', '🤝', '😅', '🫠',
]

interface Props {
  /** The counts to draw, most chosen first (the API orders them). */
  counts: ReactionCount[]
  /** Adds or takes back the caller's reaction; the parent decides which way from `counts`. */
  onToggle: (emoji: string) => void
  /**
   * Whether the reader may react at all. A signed-out reader still sees every count - reading a
   * bar never needed a credential - but has nothing to click.
   */
  canReact: boolean
  /** What this bar is attached to, for a screen reader: "post" or "comment". */
  label: string
}

/**
 * The row of emoji beneath a post or a comment.
 *
 * A chip is a toggle: pressed means you are one of the readers counted in it, and clicking it
 * again takes your own reaction back without touching anybody else's.
 */
export function ReactionBar({ counts, onToggle, canReact, label }: Props) {
  const [picking, setPicking] = useState(false)

  const react = (emoji: string) => {
    setPicking(false)
    onToggle(emoji)
  }

  // Nothing to show and nothing to click: a signed-out reader on an unreacted-to post gets no
  // empty row of furniture.
  if (counts.length === 0 && !canReact) return null

  return (
    <div className="reactions" aria-label={`Reactions on this ${label}`}>
      {counts.map((count) => (
        <button
          key={count.emoji}
          type="button"
          className={`reaction${count.reacted ? ' reaction-mine' : ''}`}
          onClick={() => react(count.emoji)}
          disabled={!canReact}
          aria-pressed={count.reacted}
          aria-label={`${count.emoji} ${count.count}`}
        >
          <span aria-hidden="true">{count.emoji}</span>
          <span className="reaction-count">{count.count}</span>
        </button>
      ))}

      {canReact && (
        <div className="reaction-picker-anchor">
          <button
            type="button"
            className="reaction reaction-add"
            onClick={() => setPicking((open) => !open)}
            aria-expanded={picking}
            aria-label={`React to this ${label}`}
          >
            <span aria-hidden="true">☺</span>
            <span aria-hidden="true">+</span>
          </button>

          {picking && (
            <div className="reaction-picker" role="menu">
              {PICKER.map((emoji) => (
                <button
                  key={emoji}
                  type="button"
                  className="reaction-choice"
                  role="menuitem"
                  onClick={() => react(emoji)}
                  aria-label={`React with ${emoji}`}
                >
                  {emoji}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
