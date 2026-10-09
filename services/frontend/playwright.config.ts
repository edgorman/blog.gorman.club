import { statSync } from 'node:fs'
import { defineConfig } from '@playwright/test'

// A Claude Code cloud session links its one Chromium binary at $PLAYWRIGHT_BROWSERS_PATH/chromium,
// and its revision needn't match this @playwright/test, so use it as-is there. CI installs the
// matching one instead.
const preinstalled = `${process.env.PLAYWRIGHT_BROWSERS_PATH}/chromium`

// The post-deploy smoke test (#277): read-only and signed out, against the live staging site.
// Named *.e2e.ts so `npm test`'s default test match never picks it up.
export default defineConfig({
  testDir: 'e2e',
  testMatch: '*.e2e.ts',
  timeout: 30_000,
  // Generous for one cold Cloud Run start, rather than a retry that would hide a real failure.
  expect: { timeout: 15_000 },
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL: process.env.SMOKE_BASE_URL ?? 'https://staging.blog.gorman.club',
    launchOptions: statSync(preinstalled, { throwIfNoEntry: false })?.isFile() ? { executablePath: preinstalled } : {},
  },
})
