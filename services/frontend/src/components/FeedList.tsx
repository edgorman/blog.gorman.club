import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import type { Blog } from '../lib/api'
import { formatDate, snippetFrom } from '../lib/format'

interface Props {
  posts: Blog[]
}

/** The recent-posts list shared by the landing feed and a profile's feed. */
export function FeedList({ posts }: Props) {
  return (
    <div>
      {posts.map((post, i) => (
        <FeedRow key={post.id} post={post} delayMs={i * 20} />
      ))}
    </div>
  )
}

function FeedRow({ post, delayMs }: { post: Blog; delayMs: number }) {
  const { resolveAuthorName } = useApp()
  const [author, setAuthor] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    resolveAuthorName(post.ownerId).then((name) => {
      if (!cancelled) setAuthor(name)
    })
    return () => {
      cancelled = true
    }
  }, [post.ownerId, resolveAuthorName])

  return (
    <Link to={`/post/${post.id}`} className="feed-row" style={{ animationDelay: `${delayMs}ms` }}>
      <div className="feed-row-meta">
        <span className="text-muted feed-row-date">{formatDate(post.createdAt)}</span>
        {author && <span className="text-muted feed-row-date">{author}</span>}
        {post.visibility === 'private' && <span className="tag tag-outline">private</span>}
      </div>
      <h2 className="feed-title">{post.title || '(untitled)'}</h2>
      <p className="text-muted feed-snippet">{snippetFrom(post.content)}</p>
    </Link>
  )
}
