import type { ReactionCount } from '../lib/api'

/**
 * The five reactions a post or comment may carry - kept in the same order the backend does
 * (`entity.AllowedEmojis`), so a bar reads the same whichever ones have been picked yet. There is
 * no custom emoji and no picker: widening this set is a change to both arrays, deliberately kept
 * in step rather than read from the API, since it is fixed either way.
 */
const REACTIONS = ['👍', '👎', '❤️', '😄', '🎉']

interface Props {
  /** The counts to draw. An emoji nobody has chosen yet is absent, not zero. */
  counts: ReactionCount[]
  /** Adds or takes back the caller's reaction; the parent decides which way from `counts`. */
  onToggle: (emoji: string) => void
  /**
   * Whether the reader may react at all. A signed-out reader still sees every count - reading a
   * bar never needed a credential - but has nothing to click, so an emoji nobody has chosen yet
   * is not shown to them either: there would be nothing to look at and nothing to do about it.
   */
  canReact: boolean
  /** What this bar is attached to, for a screen reader: "post" or "comment". */
  label: string
}

/**
 * The row of emoji beneath a post or a comment.
 *
 * A chip is a toggle: pressed means you are one of the readers counted in it, and clicking it
 * again takes your own reaction back without touching anybody else's. A signed-in reader sees all
 * five, chosen or not, so reacting is one click rather than a click to open a picker and a second
 * to choose from it.
 */
export function ReactionBar({ counts, onToggle, canReact, label }: Props) {
  const byEmoji = new Map(counts.map((count) => [count.emoji, count]))
  const shown = canReact ? REACTIONS : REACTIONS.filter((emoji) => byEmoji.has(emoji))

  // Nothing to show and nothing to click: a signed-out reader on an unreacted-to post gets no
  // empty row of furniture.
  if (shown.length === 0) return null

  return (
    <div className="reactions" aria-label={`Reactions on this ${label}`}>
      {shown.map((emoji) => {
        const count = byEmoji.get(emoji)
        return (
          <button
            key={emoji}
            type="button"
            className={`reaction${count?.reacted ? ' reaction-mine' : ''}`}
            onClick={() => onToggle(emoji)}
            disabled={!canReact}
            aria-pressed={!!count?.reacted}
            aria-label={`${emoji} ${count?.count ?? 0}`}
          >
            <span aria-hidden="true">{emoji}</span>
            <span className="reaction-count">{count?.count ?? 0}</span>
          </button>
        )
      })}
    </div>
  )
}
