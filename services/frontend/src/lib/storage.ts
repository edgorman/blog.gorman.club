/**
 * Everything the site stores in the browser. pages/Privacy.test.tsx fails if code writes a storage key
 * or cookie that isn't listed here, so the table on /privacy can't quietly fall behind the code. Every
 * item is strictly necessary or a preference the visitor set, which is why there is no consent
 * banner (#238): adding anything else means adding consent first.
 */
export const STORAGE_ITEMS = [
  {
    name: 'gc-theme',
    kind: 'localStorage',
    setBy: 'This site',
    purpose: 'Remembers whether you chose dark mode.',
    lasts: 'Until you clear it',
  },
  {
    name: 'blog.gorman.club:google-credential',
    kind: 'sessionStorage',
    setBy: 'This site',
    purpose: 'Keeps you signed in across page loads in the same tab.',
    lasts: 'Until you close the tab, sign out, or the credential expires (about an hour)',
  },
  {
    name: 'g_state and other Google sign-in cookies',
    kind: 'Cookie',
    setBy: 'Google Identity Services',
    purpose: 'Run Google sign-in and One Tap, including remembering if you dismissed the prompt.',
    lasts: "Set by Google; see Google's privacy policy",
  },
] as const
