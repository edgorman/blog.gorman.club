import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchHealth } from './health'

describe('fetchHealth', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns the parsed JSON body on success', async () => {
    const body = {
      status: 'ok',
      timestamp: '2026-08-19T00:00:00Z',
      environment: 'stag',
      commit: 'abc123',
    }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) }),
    )

    await expect(fetchHealth('https://backend.example.com')).resolves.toEqual(body)
    expect(fetch).toHaveBeenCalledWith('https://backend.example.com/debug', {
      signal: undefined,
    })
  })

  it('throws when the response is not ok', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503 }))

    await expect(fetchHealth('https://backend.example.com')).rejects.toThrow('503')
  })
})
