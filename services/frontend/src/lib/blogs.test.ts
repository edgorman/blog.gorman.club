import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('./firebase', () => ({ db: {} }))

vi.mock('firebase/firestore', () => ({
  collection: vi.fn(() => ({})),
  doc: vi.fn(() => ({})),
  getDoc: vi.fn(),
  getDocs: vi.fn(),
  query: vi.fn((_ref: unknown, ...conditions: unknown[]) => ({ conditions })),
  where: vi.fn((field: string, op: string, value: unknown) => ({ field, op, value })),
}))

import { getDocs } from 'firebase/firestore'
import { fetchVisibleBlogs } from './blogs'

function snapshotOf(entries: Array<{ id: string; createdAt: string }>) {
  return {
    docs: entries.map(({ id, createdAt }) => ({
      id,
      data: () => ({
        ownerId: 'u1',
        title: `title-${id}`,
        content: 'content',
        visibility: 'public',
        createdAt: { toDate: () => new Date(createdAt) },
        updatedAt: { toDate: () => new Date(createdAt) },
      }),
    })),
  }
}

describe('fetchVisibleBlogs', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('only queries public blogs when signed out', async () => {
    vi.mocked(getDocs).mockResolvedValueOnce(
      snapshotOf([{ id: 'a', createdAt: '2026-01-01T00:00:00Z' }]) as never,
    )

    const blogs = await fetchVisibleBlogs(null)

    expect(getDocs).toHaveBeenCalledTimes(1)
    expect(blogs.map((b) => b.id)).toEqual(['a'])
  })

  it('merges and deduplicates results across queries, sorted newest first', async () => {
    vi.mocked(getDocs)
      .mockResolvedValueOnce(
        snapshotOf([
          { id: 'a', createdAt: '2026-01-01T00:00:00Z' },
          { id: 'b', createdAt: '2026-01-02T00:00:00Z' },
        ]) as never,
      )
      .mockResolvedValueOnce(snapshotOf([{ id: 'b', createdAt: '2026-01-02T00:00:00Z' }]) as never)
      .mockResolvedValueOnce(snapshotOf([{ id: 'c', createdAt: '2026-01-03T00:00:00Z' }]) as never)

    const blogs = await fetchVisibleBlogs('u1')

    expect(getDocs).toHaveBeenCalledTimes(3)
    expect(blogs.map((b) => b.id)).toEqual(['c', 'b', 'a'])
  })
})
