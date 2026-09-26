/**
 * @jest-environment node
 */
// public/_worker.js, the Cloudflare Pages worker that serves crawlers (#221). Node rather than
// jsdom, for the fetch API's Request/Response the worker is written against.
// @ts-expect-error -- plain JS that ships to Cloudflare as-is, so it has no declarations.
import worker from '../public/_worker.js'
import { excerpt } from './lib/seo'

const SHELL = '<!doctype html><html><head><title>Gorman Club</title></head><body><div id="root"></div></body></html>'
const BACKEND = 'https://api.example.run.app'

const publicPost = {
  slug: 'hello-world',
  title: 'Hello <world>',
  content: '# Hi\n\nSome **body** text with a [link](https://x.test).',
  visibility: 'public',
  authorUsername: 'kestrel',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-02T00:00:00Z',
}

// The static assets: config.json when the deployment has one, the SPA shell for everything else.
function env(config: object | null = { backendUrl: BACKEND }) {
  return {
    ASSETS: {
      fetch: jest.fn(async (input: Request | URL) => {
        const path = new URL(input instanceof Request ? input.url : input).pathname
        if (path === '/config.json' && config) return Response.json(config)
        return new Response(SHELL, { headers: { 'Content-Type': 'text/html', 'Content-Security-Policy': "default-src 'self'" } })
      }),
    },
  }
}

// The backend as a signed-out reader sees it: one public post, everything else a 404.
function backend(pages: object[] = [{ posts: [publicPost], hasMore: false }]) {
  const calls: string[] = []
  global.fetch = jest.fn(async (input: string | URL | Request) => {
    const url = new URL(String(input))
    calls.push(url.pathname + url.search)
    if (url.pathname === `/blogs/${publicPost.slug}`) return Response.json(publicPost)
    if (url.pathname === '/blogs') return Response.json(pages[calls.filter((c) => c.startsWith('/blogs?')).length - 1])
    return Response.json({ error: 'blog not found' }, { status: 404 })
  }) as typeof fetch
  return calls
}

const get = (url: string, e = env()) => worker.fetch(new Request(url), e)

describe('_worker.js', () => {
  it('injects the post title, description, Open Graph and canonical tags into the shell', async () => {
    backend()
    const response = await get('https://blog.gorman.club/post/hello-world')
    const html = await response.text()

    expect(html).toContain('<title>Hello &lt;world&gt; · Gorman Club</title>')
    expect(html).not.toContain('<title>Gorman Club</title>')
    expect(html).toContain('<meta name="description" content="Hi Some body text with a link." />')
    expect(html).toContain('<meta property="og:title" content="Hello &lt;world&gt;" />')
    expect(html).toContain('<meta property="og:type" content="article" />')
    expect(html).toContain('<meta property="og:url" content="https://blog.gorman.club/post/hello-world" />')
    expect(html).toContain('<link rel="canonical" href="https://blog.gorman.club/post/hello-world" />')
    expect(html).toContain('"@type":"BlogPosting"')
    // The shell's own headers (the CSP from _headers) survive the rewrite.
    expect(response.headers.get('Content-Security-Policy')).toBe("default-src 'self'")
    expect(response.headers.get('X-Robots-Tag')).toBeNull()
  })

  it('leaves the shell untouched for a private or missing post, so nothing about it leaks', async () => {
    backend()
    const html = await (await get('https://blog.gorman.club/post/secret')).text()
    expect(html).toBe(SHELL)
  })

  it('leaves the shell untouched when the deployment has no backend', async () => {
    backend()
    const html = await (await get('https://blog.gorman.club/post/hello-world', env(null))).text()
    expect(html).toBe(SHELL)
  })

  it('allows crawling on prod and names the sitemap', async () => {
    const response = await get('https://blog.gorman.club/robots.txt')
    expect(await response.text()).toBe('User-agent: *\nAllow: /\nSitemap: https://blog.gorman.club/sitemap.xml\n')
  })

  it('marks every staging response noindex and disallows everything in its robots.txt', async () => {
    backend()
    const robots = await get('https://staging.blog.gorman.club/robots.txt')
    expect(await robots.text()).toBe('User-agent: *\nDisallow: /\n')
    expect(robots.headers.get('X-Robots-Tag')).toBe('noindex')

    const asset = await get('https://staging.blog.gorman.club/assets/index.js')
    expect(asset.headers.get('X-Robots-Tag')).toBe('noindex')
    expect(asset.headers.get('Content-Security-Policy')).toBe("default-src 'self'")
  })

  it('lists every public post, across pages, with its lastmod', async () => {
    const second = { ...publicPost, slug: 'older', updatedAt: '2026-07-01T00:00:00Z', createdAt: '2026-07-01T00:00:00Z' }
    const calls = backend([
      { posts: [publicPost], hasMore: true },
      { posts: [second, { ...second, slug: 'shared-with-me', visibility: 'private' }], hasMore: false },
    ])
    const response = await get('https://blog.gorman.club/sitemap.xml')
    const xml = await response.text()

    expect(response.headers.get('Content-Type')).toBe('application/xml; charset=utf-8')
    expect(calls).toEqual(['/blogs?limit=100', `/blogs?limit=100&startAfter=${encodeURIComponent(publicPost.createdAt)}`])
    expect(xml).toBe(
      [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
        '  <url><loc>https://blog.gorman.club/post/hello-world</loc><lastmod>2026-08-02T00:00:00Z</lastmod></url>',
        '  <url><loc>https://blog.gorman.club/post/older</loc><lastmod>2026-07-01T00:00:00Z</lastmod></url>',
        '</urlset>',
        '',
      ].join('\n'),
    )
  })

  // The worker keeps its own copy of excerpt(); this keeps it describing a post the way the SPA does.
  it('describes a post the same way the SPA does', async () => {
    const content = `> quote\n\n- item\n\n\`\`\`go\ncode\n\`\`\`\n\n![img](x.png) ${'word '.repeat(60)}`
    backend()
    global.fetch = jest.fn(async () => Response.json({ ...publicPost, content })) as typeof fetch
    const html = await (await get('https://blog.gorman.club/post/hello-world')).text()
    expect(html).toContain(`<meta name="description" content="${excerpt(content)}" />`)
    expect(excerpt(content).length).toBeLessThanOrEqual(160)
  })
})
