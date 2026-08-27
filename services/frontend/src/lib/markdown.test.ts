import { describe, expect, it } from 'vitest'
import { renderMarkdown } from './markdown'

describe('renderMarkdown', () => {
  it('adds slugified ids to headings so in-page anchors can target them', () => {
    const html = renderMarkdown('## Philosophy\n')

    expect(html).toContain('<h2 id="philosophy">Philosophy</h2>')
  })

  it('disambiguates duplicate headings the way GitHub does', () => {
    const html = renderMarkdown('## Philosophy\n\ntext\n\n## Philosophy\n')

    expect(html).toContain('id="philosophy"')
    expect(html).toContain('id="philosophy-1"')
  })

  it('applies language-specific syntax highlighting to fenced code blocks', () => {
    const html = renderMarkdown('```js\nconst x = 1;\n```\n')

    expect(html).toContain('class="hljs language-js"')
    expect(html).toContain('hljs-keyword')
  })

  it('leaves an unfenced or unrecognized-language block as plain escaped text', () => {
    const html = renderMarkdown('```\n<b>plain</b>\n```\n')

    expect(html).toContain('&lt;b&gt;plain&lt;/b&gt;')
    expect(html).not.toContain('hljs-')
  })
})
