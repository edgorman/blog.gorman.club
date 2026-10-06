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
  await page.goto('/')
  // Feed rows show tags as plain chips; a post's own header links them to the filtered feed.
  await feedRows(page).filter({ has: page.locator('.tag-topic') }).first().click()
  const chip = page.locator('header a.tag-topic').first()
  const tag = (await chip.textContent()) ?? ''
  await chip.click()
  await expect.poll(() => new URL(page.url()).searchParams.get('tag')).toBe(tag)
  await expect(page.getByRole('heading', { name: `Posts tagged ${tag}` })).toBeVisible()
  await expect(feedRows(page).first()).toBeVisible()
  for (const row of await feedRows(page).all()) {
    await expect(row.getByRole('list', { name: 'Tags' }).getByText(tag, { exact: true })).toBeVisible()
  }
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
