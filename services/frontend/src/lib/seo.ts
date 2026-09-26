/**
 * What search engines and link previews are told about a page (#221). The canonical origin is
 * always production's, whichever host served the page, so staging never competes with it.
 *
 * `public/_worker.js` keeps its own copy of `SITE_URL`, `SITE_NAME` and `excerpt`: it ships to
 * Cloudflare as-is rather than through Vite, so it cannot import this module. Change both together.
 */
export const SITE_URL = 'https://blog.gorman.club'
export const SITE_NAME = 'Gorman Club'
export const SITE_DESCRIPTION = 'Posts from the Gorman Club blog.'

/** A post's markdown as a plain-text description of at most `max` characters, cut on a word. */
export function excerpt(markdown: string, max = 160): string {
  const text = markdown
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/<[^>]*>/g, ' ')
    .replace(/^\s{0,3}(#{1,6}|>|[-*+]|\d+\.)\s+/gm, '')
    .replace(/[*_`~]+/g, '')
    .replace(/\s+/g, ' ')
    .trim()
  if (text.length <= max) return text
  const cut = text.slice(0, max - 1)
  return `${cut.slice(0, cut.lastIndexOf(' ') > 0 ? cut.lastIndexOf(' ') : cut.length)}…`
}
