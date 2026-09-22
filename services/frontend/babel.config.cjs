// Used by babel-jest for tests only - @vitejs/plugin-react v6 moved to oxc and no longer runs
// Babel at all, so the Vite build never reads this file (there's no plugin option to guard it
// with; the `Options` type has no `babel` field to pass one through).
module.exports = {
  presets: [
    ['@babel/preset-env', { targets: { node: 'current' } }],
    ['@babel/preset-react', { runtime: 'automatic' }],
    '@babel/preset-typescript',
  ],
  plugins: ['./babel-plugin-import-meta-env.cjs'],
}
