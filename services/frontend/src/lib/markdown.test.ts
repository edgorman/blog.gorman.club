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
})
