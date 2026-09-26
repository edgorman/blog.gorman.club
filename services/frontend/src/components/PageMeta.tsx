import { SITE_DESCRIPTION, SITE_NAME, SITE_URL } from '../lib/seo'

/**
 * A page's `<title>`, description and canonical link. React 19 hoists all three into `<head>`
 * wherever they render, and swaps them as the route changes (#221).
 */
export function PageMeta({ title, description = SITE_DESCRIPTION, path }: { title?: string; description?: string; path: string }) {
  return (
    <>
      <title>{title ? `${title} · ${SITE_NAME}` : SITE_NAME}</title>
      <meta name="description" content={description} />
      <link rel="canonical" href={`${SITE_URL}${path}`} />
    </>
  )
}
