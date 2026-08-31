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

  // The output goes straight to dangerouslySetInnerHTML, and markdown admits raw HTML in its prose,
  // so anything a post can carry runs in the reader's browser unless it is stripped here.
  describe('sanitization', () => {
    it('strips script tags from the prose', () => {
      const html = renderMarkdown('Hello\n\n<script>alert(1)</script>\n')

      expect(html).not.toContain('<script')
      expect(html).toContain('Hello')
    })

    it('strips inline event handlers, which run even when a script tag would not', () => {
      const html = renderMarkdown('<img src="x" onerror="alert(1)">\n')

      expect(html).not.toContain('onerror')
    })

    it('strips javascript: URLs from links', () => {
      const html = renderMarkdown('[click me](javascript:alert(1))\n')

      expect(html).not.toContain('javascript:')
    })

    it('keeps the formatting a post is actually written in', () => {
      const html = renderMarkdown('**bold** and [a link](https://example.com) and `code`\n')

      expect(html).toContain('<strong>bold</strong>')
      expect(html).toContain('href="https://example.com"')
      expect(html).toContain('<code>code</code>')
    })
  })
})
