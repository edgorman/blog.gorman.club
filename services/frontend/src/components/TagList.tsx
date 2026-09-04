import { Link } from 'react-router-dom'
import { tagPath } from '../lib/api'

interface Props {
  tags: string[] | undefined
  /**
   * Whether each tag links to the feed filtered by it. A feed row is itself one big link, and an
   * anchor inside an anchor is not valid HTML, so tags there are plain chips - the row already
   * goes somewhere, and the tag is there to be read rather than followed.
   */
  linked?: boolean
}

/** The topics a post is filed under, as a row of chips. Renders nothing for an untagged post. */
export function TagList({ tags, linked }: Props) {
  if (!tags || tags.length === 0) return null

  return (
    <ul className="tag-list" aria-label="Tags">
      {tags.map((tag) => (
        <li key={tag}>
          {linked ? (
            <Link to={tagPath(tag)} className="tag tag-topic">
              {tag}
            </Link>
          ) : (
            <span className="tag tag-topic">{tag}</span>
          )}
        </li>
      ))}
    </ul>
  )
}
