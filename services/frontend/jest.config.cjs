module.exports = {
  testEnvironment: 'jest-environment-jsdom',
  setupFiles: ['<rootDir>/jest.setup-env.cjs'],
  setupFilesAfterEnv: ['<rootDir>/src/setupTests.ts'],
  transform: {
    '^.+\\.(t|j)sx?$': 'babel-jest',
  },
  // marked ships ESM-only (no "require" export condition - see its package.json), so it has to be
  // transformed like the app's own source rather than left as-is like the rest of node_modules.
  transformIgnorePatterns: ['node_modules/(?!(marked)/)'],
}
