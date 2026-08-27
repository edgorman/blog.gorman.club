import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import go from 'highlight.js/lib/languages/go'
import json from 'highlight.js/lib/languages/json'
import markdownLang from 'highlight.js/lib/languages/markdown'
import python from 'highlight.js/lib/languages/python'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import { marked, type Tokens } from 'marked'
import { markedHighlight } from 'marked-highlight'

// A focused subset covering the languages this blog actually uses in code fences, rather than
// hljs's full ~190-language bundle.
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('css', css)
hljs.registerLanguage('go', go)
hljs.registerLanguage('json', json)
hljs.registerLanguage('markdown', markdownLang)
hljs.registerLanguage('python', python)
// TypeScript's highlighter also covers plain JS/JSX/TSX fences.
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('javascript', typescript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('js', typescript)
hljs.registerLanguage('jsx', typescript)
hljs.registerLanguage('tsx', typescript)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('yaml', yaml)

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

marked.use(
  markedHighlight({
    // marked itself never colors code fences - it only tags them with a `language-xxx` class - so
    // without a highlighter wired in here, every ```js block renders as plain, uncolored text.
    langPrefix: 'hljs language-',
    highlight(code, lang) {
      // An unset or unrecognized language is left untouched - marked falls back to its own
      // (safe) escaping for the token, same as before a highlighter was wired in.
      if (!hljs.getLanguage(lang)) return code
      return hljs.highlight(code, { language: lang }).value
    },
  }),
)

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
