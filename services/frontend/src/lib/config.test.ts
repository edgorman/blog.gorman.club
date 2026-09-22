import { resolveConfig } from './config'

function mockFetch(impl: () => Promise<Partial<Response>>) {
  const fetchMock = jest.fn(impl)
  globalThis.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
})

describe('resolveConfig', () => {
  it('uses backendUrl, version, and environment from config.json when present', async () => {
    mockFetch(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({ backendUrl: 'https://config.example.com', version: 'v1.2.3', environment: 'staging' }),
      }),
    )

    await expect(resolveConfig()).resolves.toEqual({
      backendUrl: 'https://config.example.com',
      version: 'v1.2.3',
      environment: 'staging',
    })
  })

  // vite.config.ts sets VITE_BACKEND_URL to http://api.test for every test in this suite.
  it('falls back to VITE_BACKEND_URL, with no version or environment, when config.json is missing (404)', async () => {
    mockFetch(() => Promise.resolve({ ok: false, status: 404 }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test' })
  })

  it('falls back to VITE_BACKEND_URL when config.json is not valid JSON', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.reject(new Error('bad json')) }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test' })
  })

  it('falls back to VITE_BACKEND_URL when config.json is empty', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test' })
  })

  it('falls back to VITE_BACKEND_URL when backendUrl is not a non-empty string', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.resolve({ backendUrl: '' }) }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test' })
  })

  it('falls back to VITE_BACKEND_URL when the fetch itself fails', async () => {
    mockFetch(() => Promise.reject(new Error('network error')))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test' })
  })

  it('leaves version and environment undefined when they are not non-empty strings', async () => {
    mockFetch(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ backendUrl: 'https://x.test', version: '', environment: 42 }) }),
    )

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'https://x.test' })
  })
})
