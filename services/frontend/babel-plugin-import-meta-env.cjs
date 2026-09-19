/**
 * Rewrites `import.meta.env` to `process.env` so Babel's CommonJS output (which Jest runs) can
 * evaluate it - `import.meta` is only valid in an actual ES module, and Jest runs tests as CJS via
 * babel-jest regardless of this package's "type": "module". Vite replaces `import.meta.env` itself
 * at build time, so production behaviour is untouched; this only applies to the Jest transform.
 */
module.exports = function importMetaEnvPlugin() {
  return {
    visitor: {
      MemberExpression(path) {
        const { object, property } = path.node
        if (property.type !== 'Identifier' || property.name !== 'env') return
        if (object.type !== 'MetaProperty') return
        if (object.meta.name !== 'import' || object.property.name !== 'meta') return

        path.replaceWithSourceString('process.env')
      },
    },
  }
}
