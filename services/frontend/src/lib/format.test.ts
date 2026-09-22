import { formatVersion } from './format'

describe('formatVersion', () => {
  it('shortens a full commit SHA to 7 characters', () => {
    expect(formatVersion('a1b2c3d4e5f60718293a4b5c6d7e8f9012345678')).toBe('a1b2c3d')
  })

  it('leaves a release tag as-is', () => {
    expect(formatVersion('v1.2.3')).toBe('v1.2.3')
  })
})
