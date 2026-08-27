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
