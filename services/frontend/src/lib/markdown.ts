import { marked } from 'marked'

/** Renders markdown synchronously - safe as long as no async marked extension is registered. */
export function renderMarkdown(markdown: string): string {
  return marked.parse(markdown, { async: false })
}
