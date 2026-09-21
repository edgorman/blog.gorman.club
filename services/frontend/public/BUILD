# Vite's public/ directory - copied into dist/ as-is rather than bundled, which is exactly why
# nothing else in the tree ever `import`s theme.js (it's a classic script tag in index.html, not a
# module). `pants tailor --check` still wants an owning target for it regardless of whether
# anything imports it - this is that target, added once #155 turned tailor-check on in CI and it
# surfaced this directory as the one pre-existing gap.
javascript_sources()
