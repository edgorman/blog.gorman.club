import { releaseUrl } from './format'

describe('releaseUrl', () => {
  it('links to the tag\'s GitHub Releases page', () => {
    expect(releaseUrl('v1.2.3')).toBe('https://github.com/edgorman/blog.gorman.club/releases/tag/v1.2.3')
  })

  it('links the short commit SHA staging runs to the commit', () => {
    expect(releaseUrl('abc1234')).toBe('https://github.com/edgorman/blog.gorman.club/commit/abc1234')
  })
})
