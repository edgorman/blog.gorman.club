import '@testing-library/jest-dom/vitest'

// jsdom has no matchMedia; useTheme reads the OS colour scheme through it. Light, never changes.
// The worker tests run in node, where there is no window.
if (typeof window !== 'undefined') window.matchMedia ??= (query: string) =>
  ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }) as unknown as MediaQueryList
