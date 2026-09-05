/**
 * The two directions between the editor's tags field - one line of text - and the list of tags the
 * API takes. Normalization itself is the backend's (`entity.NormalizeTag`): what an author types
 * is only split up here, and comes back in its stored form once the post is saved.
 */

/**
 * How many tags one post may carry, kept in step with the backend's `entity.MaxTags` - which is
 * what actually enforces it. This is here so the editor can say the number before a save is
 * refused for exceeding it.
 */
export const MAX_TAGS = 10

/**
 * Splits what an author typed into the tags to send. Commas separate tags, so the words within one
 * are left alone - "web dev, go" is two tags, not three - and blanks are dropped so a trailing
 * comma is harmless.
 */
export function parseTags(input: string): string[] {
  return input
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
}

/** A post's stored tags as the one line the field edits. */
export function formatTags(tags: string[] | undefined): string {
  return (tags ?? []).join(', ')
}
