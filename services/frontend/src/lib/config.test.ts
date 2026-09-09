import { afterEach, describe, expect, it, vi } from 'vitest'
import { resolveBackendUrl } from './config'

function mockFetch(impl: () => Promise<Partial<Response>>) {
  const fetchMock = vi.fn(impl)
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => vi.unstubAllGlobals())

describe('resolveBackendUrl', () => {
  it('uses backendUrl from config.json when present', async () => {
    mockFetch(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ backendUrl: 'https://config.example.com' }) }),
    )

    await expect(resolveBackendUrl()).resolves.toBe('https://config.example.com')
  })

  // vite.config.ts sets VITE_BACKEND_URL to http://api.test for every test in this suite.
  it('falls back to VITE_BACKEND_URL when config.json is missing (404)', async () => {
    mockFetch(() => Promise.resolve({ ok: false, status: 404 }))

    await expect(resolveBackendUrl()).resolves.toBe('http://api.test')
  })

  it('falls back to VITE_BACKEND_URL when config.json is not valid JSON', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.reject(new Error('bad json')) }))

    await expect(resolveBackendUrl()).resolves.toBe('http://api.test')
  })

  it('falls back to VITE_BACKEND_URL when config.json has no backendUrl field', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }))

    await expect(resolveBackendUrl()).resolves.toBe('http://api.test')
  })

  it('falls back to VITE_BACKEND_URL when backendUrl is not a non-empty string', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.resolve({ backendUrl: '' }) }))

    await expect(resolveBackendUrl()).resolves.toBe('http://api.test')
  })

  it('falls back to VITE_BACKEND_URL when the fetch itself fails', async () => {
    mockFetch(() => Promise.reject(new Error('network error')))

    await expect(resolveBackendUrl()).resolves.toBe('http://api.test')
  })
})
