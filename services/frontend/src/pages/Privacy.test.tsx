import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { screen } from '@testing-library/react'
import { renderWithApp } from '../testUtils'
import { STORAGE_ITEMS } from '../lib/storage'
import { Privacy } from './Privacy'

const ROOT = join(__dirname, '..', '..')

/** Every shipped source file: src/ and public/, minus tests and generated code. */
function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) return entry.name === 'gen' ? [] : sources(path)
    return /\.(ts|tsx|js)$/.test(entry.name) && !/\.test\./.test(entry.name) ? [path] : []
  })
}

/** Each key read or written through localStorage/sessionStorage, resolving a const it is named by. */
function storageKeys(code: string, file: string): string[] {
  return [...code.matchAll(/(?:local|session)Storage\.(?:getItem|setItem|removeItem)\(\s*([^,)]+)/g)].map(([, arg]) => {
    const literal = /^['"`]([^'"`]+)['"`]$/.exec(arg.trim())?.[1]
    const named = new RegExp(`const ${arg.trim()} = ['"\`]([^'"\`]+)['"\`]`).exec(code)?.[1]
    const key = literal ?? named
    if (!key) throw new Error(`${file}: can't resolve the storage key ${arg.trim()}; use a literal or a const`)
    return key
  })
}

// The "no consent banner" answer in #238 holds only while everything stored is in the notice.
describe('browser storage guard', () => {
  const files = [...sources(join(ROOT, 'src')), ...sources(join(ROOT, 'public'))]
  const documented = STORAGE_ITEMS.map((item) => item.name)

  it('finds the storage the site is known to use', () => {
    const found = files.flatMap((file) => storageKeys(readFileSync(file, 'utf8'), file))
    expect(new Set(found)).toEqual(new Set(['gc-theme', 'blog.gorman.club:google-credential']))
  })

  it.each(files)('%s only uses storage keys listed on the privacy page', (file) => {
    for (const key of storageKeys(readFileSync(file, 'utf8'), file)) expect(documented).toContain(key)
  })

  it.each(files)('%s never writes a cookie itself', (file) => {
    expect(readFileSync(file, 'utf8')).not.toMatch(/document\.cookie/)
  })

  it('catches a storage key the page does not list', () => {
    const key = storageKeys("const KEY = 'tracker'\nlocalStorage.setItem(KEY, '1')", 'x.ts')[0]
    expect(documented).not.toContain(key)
  })
})

describe('Privacy', () => {
  it('lists every storage item and names every processor', () => {
    renderWithApp(<Privacy />)
    for (const item of STORAGE_ITEMS) expect(screen.getByText(item.name)).toBeInTheDocument()
    for (const processor of ['Google Cloud', 'Google Identity Services', 'Cloudflare', 'Stripe']) {
      expect(screen.getAllByText(processor, { selector: 'strong' }).length).toBeGreaterThan(0)
    }
  })
})
