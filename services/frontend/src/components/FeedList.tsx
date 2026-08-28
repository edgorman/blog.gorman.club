import { Link } from 'react-router-dom'
import { postPath, type Blog } from '../lib/api'
import { formatDate, snippetFrom } from '../lib/format'

interface Props {
  posts: Blog[]
}

/** The recent-posts list shared by the landing feed and a profile's feed. */
export function FeedList({ posts }: Props) {
  return (
    <div>
      {posts.map((post, i) => (
        // A slug is unique to its author, so the author is part of what identifies a row.
        <FeedRow key={`${post.ownerId}/${post.slug}`} post={post} delayMs={i * 20} />
      ))}
    </div>
  )
}

function FeedRow({ post, delayMs }: { post: Blog; delayMs: number }) {
  const author = post.authorUsername
  // A post is addressed through its author, so one whose owner holds no username has nowhere to
  // link to. It still belongs in the feed - it is simply not clickable.
  const href = postPath(post)

  const body = (
    <>
      <div className="feed-row-meta">
        <span className="text-muted feed-row-date">{formatDate(post.createdAt)}</span>
        {author && <span className="text-muted feed-row-author">{author}</span>}
        {post.visibility === 'private' && <span className="tag tag-outline">private</span>}
      </div>
      <h2 className="feed-title">{post.title || '(untitled)'}</h2>
      <p className="text-muted feed-snippet">{snippetFrom(post.content)}</p>
    </>
  )
  const style = { animationDelay: `${delayMs}ms` }

  if (!href) {
    return (
      <div className="feed-row" style={style}>
        {body}
      </div>
    )
  }
  return (
    <Link to={href} className="feed-row" style={style}>
      {body}
    </Link>
  )
}
