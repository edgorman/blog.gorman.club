// jest-environment-jsdom's global doesn't include these (unlike Node's own global or a browser),
// but react-router and useGoogleAuth.ts's credential decoding both need them.
const { TextEncoder, TextDecoder } = require('node:util')
if (typeof globalThis.TextEncoder === 'undefined') globalThis.TextEncoder = TextEncoder
if (typeof globalThis.TextDecoder === 'undefined') globalThis.TextDecoder = TextDecoder

// Mirrors vite.config.ts's old `test.env` block: AppProvider and config.ts fall back to this when
// no backendUrl/config.json is available, which is what their tests rely on outside of the cases
// that stub it themselves (see hooks/useGoogleAuth.test.ts for VITE_GOOGLE_CLIENT_ID).
process.env.VITE_BACKEND_URL = 'http://api.test'
