#!/bin/bash
set -uo pipefail

# PostToolUse hook: formats a file Claude Code just wrote or edited with the same formatter CI's
# `pants lint` enforces for it - gofmt for Go, `buf format` for protos - so formatting is fixed at
# edit time rather than discovered as a red CI run. Anything else, or a formatter that isn't
# installed, is left alone: this is a convenience, never a gate, so it always exits 0.

command -v jq >/dev/null 2>&1 || exit 0
file="$(jq -r '.tool_input.file_path // empty')"
[ -n "$file" ] && [ -f "$file" ] || exit 0

case "$file" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$file"
    ;;
  *.proto)
    # buf resolves the module (packages/protos/buf.yaml) from its working directory.
    command -v buf >/dev/null 2>&1 && (cd "$CLAUDE_PROJECT_DIR/packages/protos" && buf format -w "$file")
    ;;
esac
exit 0
