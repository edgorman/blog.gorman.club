import { describe, expect, it } from 'vitest'
import { tagPath } from './api'
import { formatTags, parseTags } from './tags'

describe('parseTags', () => {
  // Commas separate tags, so the words within one are left alone - normalizing "web dev" into
  // "web-dev" is the backend's job, and doing it here too would be a second definition of it.
  it('splits on commas and leaves the words within a tag alone', () => {
    expect(parseTags('go, web dev')).toEqual(['go', 'web dev'])
  })

  it('trims and drops blanks, so a trailing comma is harmless', () => {
    expect(parseTags('  go ,, web dev,  ')).toEqual(['go', 'web dev'])
  })

  it('reads an empty field as no tags', () => {
    expect(parseTags('')).toEqual([])
    expect(parseTags('   ')).toEqual([])
  })
})

describe('formatTags', () => {
  it('renders stored tags as the one line the field edits', () => {
    expect(formatTags(['go', 'web-dev'])).toBe('go, web-dev')
  })

  // A post with no tags at all carries no `tags` field, which the editor still has to render.
  it('renders an untagged post as an empty field', () => {
    expect(formatTags(undefined)).toBe('')
    expect(formatTags([])).toBe('')
  })

  it('round trips through parseTags', () => {
    expect(parseTags(formatTags(['go', 'web-dev']))).toEqual(['go', 'web-dev'])
  })
})

describe('tagPath', () => {
  it('points at the feed narrowed to that tag', () => {
    expect(tagPath('web-dev')).toBe('/?tag=web-dev')
  })

  // Tags admit any script, so a tag is escaped like any other value put in a query string.
  it('escapes the tag', () => {
    expect(tagPath('日本語')).toBe('/?tag=%E6%97%A5%E6%9C%AC%E8%AA%9E')
  })
})
