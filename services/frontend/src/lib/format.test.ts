import { releaseUrl } from './format'

describe('releaseUrl', () => {
  it('links to the tag\'s GitHub Releases page', () => {
    expect(releaseUrl('v1.2.3')).toBe('https://github.com/edgorman/blog.gorman.club/releases/tag/v1.2.3')
  })
})
