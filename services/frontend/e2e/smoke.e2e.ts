import { expect, test, type Page } from '@playwright/test'

const feedRows = (page: Page) => page.locator('a.feed-row')

test('the feed loads and shows at least one post', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Recent posts' })).toBeVisible()
  await expect(feedRows(page).first()).toBeVisible()
})

test('opening a post renders its body and the comment section', async ({ page }) => {
  await page.goto('/')
  const first = feedRows(page).first()
  const title = (await first.locator('.feed-title').textContent()) ?? ''
  await first.click()
  await expect(page).toHaveURL(/\/post\/[^/]+$/)
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible()
  await expect(page.locator('.post-body')).not.toBeEmpty()
  await expect(page.getByRole('region', { name: 'Comments' })).toBeVisible()
})

test('the tag filter changes the feed and the URL', async ({ page }) => {
  // A tag no post carries, rather than one read off the feed: staging's posts needn't be tagged
  // at all, and an empty feed still proves the backend applied the filter.
  const tag = 'smoke-test-no-such-tag'
  await page.goto(`/?tag=${tag}`)
  await expect(page.getByRole('heading', { name: `Posts tagged ${tag}` })).toBeVisible()
  await expect(page.getByText('No posts match that.')).toBeVisible()
  await page.getByRole('button', { name: 'Clear' }).click()
  await expect.poll(() => new URL(page.url()).searchParams.has('tag')).toBe(false)
  await expect(page.getByRole('heading', { name: 'Recent posts' })).toBeVisible()
  await expect(feedRows(page).first()).toBeVisible()
})

test('the search box changes the feed and the URL', async ({ page }) => {
  await page.goto('/')
  const title = ((await feedRows(page).first().locator('.feed-title').textContent()) ?? '').trim()
  await page.getByRole('searchbox', { name: 'Search posts' }).fill(title)
  await page.getByRole('button', { name: 'Search' }).click()
  await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe(title.trim())
  await expect(page.getByRole('heading', { name: `Best matches for “${title}”` })).toBeVisible()
  await expect(feedRows(page).filter({ hasText: title }).first()).toBeVisible()
})

test('the sign-in button renders', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Account' }).click()
  const panel = page.getByRole('dialog', { name: 'Account' })
  await expect(panel.getByRole('alert')).toHaveCount(0)
  // Google Identity Services draws the button into an iframe of its own.
  await expect(panel.locator('iframe[src*="accounts.google.com"]')).toBeVisible()
})

test('/sitemap.xml is valid XML', async ({ page, request }) => {
  const response = await request.get('/sitemap.xml')
  expect(response.ok()).toBe(true)
  const xml = await response.text()
  const root = await page.evaluate((text) => {
    const doc = new DOMParser().parseFromString(text, 'application/xml')
    return doc.querySelector('parsererror') ? null : doc.documentElement.localName
  }, xml)
  expect(root).toBe('urlset')
  expect(xml).toContain('<loc>')
})
