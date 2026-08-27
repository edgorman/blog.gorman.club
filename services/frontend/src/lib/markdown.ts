import { marked, type Tokens } from 'marked'

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\- ]+/g, '')
    .replace(/\s+/g, '-')
}

const usedSlugs = new Set<string>()

function uniqueSlug(slug: string): string {
  if (!usedSlugs.has(slug)) {
    usedSlugs.add(slug)
    return slug
  }
  let i = 1
  while (usedSlugs.has(`${slug}-${i}`)) i++
  const unique = `${slug}-${i}`
  usedSlugs.add(unique)
  return unique
}

marked.use({
  renderer: {
    heading(token: Tokens.Heading) {
      const text = this.parser.parseInline(token.tokens, this.parser.textRenderer)
      const id = uniqueSlug(slugify(text))
      return `<h${token.depth} id="${id}">${this.parser.parseInline(token.tokens)}</h${token.depth}>\n`
    },
  },
})

/** Renders markdown synchronously - safe as long as no async marked extension is registered. */
export function renderMarkdown(markdown: string): string {
  usedSlugs.clear()
  return marked.parse(markdown, { async: false })
}
