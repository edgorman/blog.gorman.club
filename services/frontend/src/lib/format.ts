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

const FULL_COMMIT_SHA = /^[0-9a-f]{40}$/

/**
 * A release tag (e.g. "v1.2.3", written into staging and production's config.json alike once a
 * commit has one - see the Versioning section of CLAUDE.md) displays as-is; a raw commit SHA -
 * what staging's config.json carries ahead of its own release tag being cut, since the version is
 * only calculated after staging's deploy (see Pre-Release Generation in CLAUDE.md) - shortens to
 * 7 characters, matching how the release notes job already truncates a commit SHA for display.
 */
export function formatVersion(version: string): string {
  return FULL_COMMIT_SHA.test(version) ? version.slice(0, 7) : version
}
