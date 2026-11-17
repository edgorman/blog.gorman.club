const DATE_FORMAT = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
})

/** e.g. "Aug 24, 2026", matching the feed's date label. */
export function formatDate(iso: string): string {
  return DATE_FORMAT.format(new Date(iso))
}

const SNIPPET_LENGTH = 160

/** Strips the most common markdown syntax and truncates, for a feed-row preview of the content. */
export function snippetFrom(content: string): string {
  const plain = content
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)]\([^)]*\)/g, '$1')
    .replace(/[#>*_`~-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

  if (plain.length <= SNIPPET_LENGTH) return plain
  return `${plain.slice(0, SNIPPET_LENGTH).trimEnd()}…`
}

export const GITHUB_REPO = 'edgorman/blog.gorman.club'

/**
 * Links what config.json says is deployed (written by the deploy pipeline - see `.github/AGENTS.md`)
 * to its GitHub page: a release tag (e.g. "v1.2.3", what production runs) to its Releases page, and
 * the short commit SHA staging runs, which is not a tag, to the commit.
 */
export function releaseUrl(version: string): string {
  return /^v\d/.test(version)
    ? `https://github.com/${GITHUB_REPO}/releases/tag/${version}`
    : `https://github.com/${GITHUB_REPO}/commit/${version}`
}
