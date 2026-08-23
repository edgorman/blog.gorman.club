import {
  collection,
  doc,
  getDoc,
  getDocs,
  query,
  where,
  type DocumentData,
} from 'firebase/firestore'
import { db } from './firebase'

export interface Blog {
  id: string
  ownerId: string
  title: string
  content: string
  visibility: 'public' | 'private'
  allowedUserIds?: string[]
  createdAt: string
  updatedAt: string
}

function fromData(id: string, data: DocumentData): Blog {
  return {
    id,
    ownerId: data.ownerId as string,
    title: data.title as string,
    content: data.content as string,
    visibility: data.visibility as Blog['visibility'],
    allowedUserIds: data.allowedUserIds as string[] | undefined,
    createdAt: data.createdAt?.toDate?.().toISOString() ?? '',
    updatedAt: data.updatedAt?.toDate?.().toISOString() ?? '',
  }
}

/**
 * Fetches every blog visible to the caller: public posts, the caller's own posts, and private
 * posts where the caller is in allowedUserIds. This mirrors firestore.rules' read condition for
 * /blogs/{blogId}, which is what actually enforces it - these queries just ask for what's already
 * allowed. Firestore has no single query that ORs across all three, so they're run separately and
 * merged, deduplicating by ID.
 */
export async function fetchVisibleBlogs(uid: string | null): Promise<Blog[]> {
  if (!db) return []

  const blogsRef = collection(db, 'blogs')
  const queries = [query(blogsRef, where('visibility', '==', 'public'))]

  if (uid) {
    queries.push(query(blogsRef, where('ownerId', '==', uid)))
    queries.push(query(blogsRef, where('allowedUserIds', 'array-contains', uid)))
  }

  const snapshots = await Promise.all(queries.map((q) => getDocs(q)))

  const byId = new Map<string, Blog>()
  for (const snapshot of snapshots) {
    for (const document of snapshot.docs) {
      byId.set(document.id, fromData(document.id, document.data()))
    }
  }

  return [...byId.values()].sort((a, b) => b.createdAt.localeCompare(a.createdAt))
}

export async function fetchBlog(id: string): Promise<Blog | null> {
  if (!db) return null

  const snapshot = await getDoc(doc(db, 'blogs', id))
  return snapshot.exists() ? fromData(snapshot.id, snapshot.data()) : null
}
