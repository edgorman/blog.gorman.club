// Used by babel-jest for tests only - the Vite build never reads this file (see the `babel: {
// babelrc: false, configFile: false }` option passed to @vitejs/plugin-react in vite.config.ts).
module.exports = {
  presets: [
    ['@babel/preset-env', { targets: { node: 'current' } }],
    ['@babel/preset-react', { runtime: 'automatic' }],
    '@babel/preset-typescript',
  ],
  plugins: ['./babel-plugin-import-meta-env.cjs'],
}
