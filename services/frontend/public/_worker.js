// Cloudflare Pages advanced-mode worker (#221): what crawlers and link previews need that a
// JavaScript-only SPA can't give them. It lives in public/ so Vite copies it into dist/, which makes
// it part of the commit-SHA bundle frontend-publish stores and prod promotes unchanged. Everything
// it doesn't handle goes to the static assets untouched, _headers and all.
//
// It finds the backend the same way the SPA does, from the config.json frontend-deploy writes next
// to the bundle, and only ever calls it anonymously, so it sees exactly what a signed-out reader
// can: a private post is a 404 to it, and its title never reaches the page.
//
// SITE_URL, SITE_NAME and excerpt() mirror src/lib/seo.ts, which this file can't import because it
// ships as-is rather than through Vite. Change both together.

const SITE_URL = 'https://blog.gorman.club'
const SITE_NAME = 'Gorman Club'
const PROD_HOSTNAME = new URL(SITE_URL).hostname
// ponytail: stops the sitemap at 50 pages of 100 posts; move to a dedicated backend route if the
// blog ever outgrows that.
const SITEMAP_MAX_PAGES = 50
// How long the backend gets to answer all its pages before the sitemap falls back (#246).
const SITEMAP_TIMEOUT_MS = 10_000
// How long a colo keeps the last good sitemap to fall back to: a week.
const SITEMAP_LAST_GOOD_SECONDS = 7 * 24 * 60 * 60
// The SPA's routes that aren't posts or profiles but are worth indexing.
const STATIC_PAGES = ['/privacy', '/terms']

export default {
  async fetch(request, env) {
    const url = new URL(request.url)
    const prod = url.hostname === PROD_HOSTNAME
    const response = await route(request, env, url, prod)
    if (prod) return response
    // Staging (or a *.pages.dev preview) serves the same bundle and the same _headers as prod, so
    // this is the one place that can tell it apart and keep it out of search results.
    const marked = new Response(response.body, response)
    marked.headers.set('X-Robots-Tag', 'noindex')
    return marked
  },
}

async function route(request, env, url, prod) {
  if (url.pathname === '/robots.txt') {
    const body = prod
      ? `User-agent: *\nAllow: /\nSitemap: ${SITE_URL}/sitemap.xml\n`
      : 'User-agent: *\nDisallow: /\n'
    return new Response(body, { headers: { 'Content-Type': 'text/plain; charset=utf-8' } })
  }
  if (url.pathname === '/sitemap.xml') return sitemap(env, url)
  const slug = /^\/post\/([^/]+)\/?$/.exec(url.pathname)?.[1]
  if (slug && slug !== 'new' && request.method === 'GET') return postShell(request, env, url, slug)
  return env.ASSETS.fetch(request)
}

/** The SPA shell with the post's title, description, Open Graph and canonical tags in its head. */
async function postShell(request, env, url, encodedSlug) {
  const [shell, post] = await Promise.all([env.ASSETS.fetch(request), fetchPost(env, url, encodedSlug)])
  if (!post || shell.status !== 200) return shell
  const headers = new Headers(shell.headers)
  headers.delete('Content-Length')
  headers.delete('ETag')
  const html = (await shell.text()).replace(/<title>[^<]*<\/title>/, postTags(post))
  return new Response(html, { status: 200, headers })
}

/** The post as a signed-out reader sees it, or null for a private, missing or unreachable one. */
async function fetchPost(env, url, encodedSlug) {
  try {
    const backend = await backendUrl(env, url)
    if (!backend) return null
    const slug = decodeURIComponent(encodedSlug)
    const response = await fetch(`${backend}/blogs/${encodeURIComponent(slug)}`)
    if (!response.ok) return null
    const post = await response.json()
    return post.visibility === 'public' ? post : null
  } catch {
    return null
  }
}

/**
 * Always a 200 sitemap (#246): Search Console reports any error, or anything that isn't XML, as
 * "Sitemap could not be read", so a backend that's down, cold or slow falls back to the last good
 * sitemap this colo cached, or failing that to the static pages alone, rather than a 502.
 */
async function sitemap(env, url) {
  const cache = typeof caches === 'undefined' ? null : caches.default
  const lastGood = new Request(new URL('/sitemap.xml?last-good', url))
  try {
    const xml = sitemapXml(await sitemapEntries(env, url))
    const response = xmlResponse(xml, 3600)
    // Kept well past the hour it's served fresh, so there's something to fall back to.
    await cache?.put(lastGood, xmlResponse(xml, SITEMAP_LAST_GOOD_SECONDS)).catch(() => {})
    return response
  } catch (e) {
    console.error(`Sitemap falling back: ${e instanceof Error ? e.message : e}`)
    const cached = await cache?.match(lastGood).catch(() => undefined)
    if (cached) return xmlResponse(await cached.text(), 300)
    return xmlResponse(sitemapXml(STATIC_PAGES.map(staticEntry)), 300)
  }
}

/** Every <url> line: the static pages, then each public post. Throws if the backend can't be read. */
async function sitemapEntries(env, url) {
  const backend = await backendUrl(env, url)
  if (!backend) throw new Error('no backend configured')
  const entries = STATIC_PAGES.map(staticEntry)
  // One deadline for all the pages, so a Cloud Run cold start falls back before the crawler gives up.
  const signal = AbortSignal.timeout(SITEMAP_TIMEOUT_MS)
  let startAfter = ''
  for (let i = 0; i < SITEMAP_MAX_PAGES; i++) {
    const query = new URLSearchParams({ limit: '100' })
    if (startAfter) query.set('startAfter', startAfter)
    const response = await fetch(`${backend}/blogs?${query}`, { signal })
    if (!response.ok) throw new Error(`GET /blogs answered ${response.status}`)
    const page = await response.json()
    const posts = page.posts ?? []
    for (const post of posts) {
      if (post.visibility !== 'public') continue
      const lastmod = post.updatedAt ? `<lastmod>${escapeHtml(post.updatedAt)}</lastmod>` : ''
      entries.push(`  <url><loc>${escapeHtml(postUrl(post))}</loc>${lastmod}</url>`)
    }
    if (!page.hasMore || posts.length === 0) break
    startAfter = posts.at(-1).createdAt
  }
  return entries
}

function staticEntry(path) {
  return `  <url><loc>${SITE_URL}${path}</loc></url>`
}

function sitemapXml(entries) {
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...entries,
    '</urlset>',
    '',
  ].join('\n')
}

function xmlResponse(xml, maxAge) {
  return new Response(xml, {
    headers: { 'Content-Type': 'application/xml; charset=utf-8', 'Cache-Control': `public, max-age=${maxAge}` },
  })
}

/** config.json's backendUrl, or '' when this deployment has none. */
async function backendUrl(env, url) {
  try {
    const response = await env.ASSETS.fetch(new URL('/config.json', url))
    const config = response.ok ? await response.json() : {}
    return typeof config.backendUrl === 'string' ? config.backendUrl.replace(/\/$/, '') : ''
  } catch {
    // Missing: Pages answers the SPA shell instead, which isn't JSON.
    return ''
  }
}

function postUrl(post) {
  return `${SITE_URL}/post/${encodeURIComponent(post.slug)}`
}

function postTags(post) {
  const title = post.title || '(untitled)'
  const description = excerpt(post.content ?? '')
  const href = postUrl(post)
  const jsonLd = JSON.stringify({
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    headline: title,
    description,
    url: href,
    ...(post.createdAt ? { datePublished: post.createdAt } : {}),
    ...(post.updatedAt ? { dateModified: post.updatedAt } : {}),
    ...(post.authorUsername ? { author: { '@type': 'Person', name: post.authorUsername } } : {}),
  }).replace(/</g, '\\u003c')
  return [
    `<title>${escapeHtml(title)} · ${SITE_NAME}</title>`,
    `<meta name="description" content="${escapeHtml(description)}" />`,
    `<link rel="canonical" href="${escapeHtml(href)}" />`,
    `<meta property="og:type" content="article" />`,
    `<meta property="og:site_name" content="${SITE_NAME}" />`,
    `<meta property="og:title" content="${escapeHtml(title)}" />`,
    `<meta property="og:description" content="${escapeHtml(description)}" />`,
    `<meta property="og:url" content="${escapeHtml(href)}" />`,
    `<meta name="twitter:card" content="summary" />`,
    `<script type="application/ld+json">${jsonLd}</script>`,
  ].join('\n    ')
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function excerpt(markdown, max = 160) {
  const text = markdown
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/<[^>]*>/g, ' ')
    .replace(/^\s{0,3}(#{1,6}|>|[-*+]|\d+\.)\s+/gm, '')
    .replace(/[*_`~]+/g, '')
    .replace(/\s+/g, ' ')
    .trim()
  if (text.length <= max) return text
  const cut = text.slice(0, max - 1)
  return `${cut.slice(0, cut.lastIndexOf(' ') > 0 ? cut.lastIndexOf(' ') : cut.length)}…`
}
