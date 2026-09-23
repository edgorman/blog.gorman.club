#!/bin/bash
set -uo pipefail

# Installs the plugins .claude/settings.json turns on (`enabledPlugins`) in a Claude Code cloud
# session, which doesn't install them itself: a cloud session reads a repository's
# `enabledPlugins`/`extraKnownMarketplaces` but installs nothing from them (see
# https://code.claude.com/docs/en/cloud-environments#what-carries-over-from-your-setup). Locally,
# Claude Code offers the same install once you trust the folder, so this is cloud-only.
#
# settings.json stays the one list: this reads the marketplaces and plugins back out of it rather
# than naming them a second time. Plugins install at user scope, into the session's throwaway
# ~/.claude, so nothing here rewrites the committed settings file.
#
# A SessionStart hook runs after Claude Code has loaded its plugins, so a plugin this installs is
# active from the next start or resume in the same container, or after `/reload-plugins`. Best-effort
# throughout: a marketplace that can't be reached must not stop the session starting, so this
# always exits 0.
if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi
command -v claude >/dev/null 2>&1 && command -v jq >/dev/null 2>&1 || exit 0

settings="$CLAUDE_PROJECT_DIR/.claude/settings.json"
known="$(claude plugin marketplace list 2>/dev/null)"
installed="$(claude plugin list 2>/dev/null)"

# claude-plugins-official is added automatically only on an interactive first start, which a
# cloud session isn't, so it's added here alongside the ones settings.json declares.
{
  echo "claude-plugins-official anthropics/claude-plugins-official"
  jq -r '.extraKnownMarketplaces // {} | to_entries[]
    | select(.value.source.source == "github") | "\(.key) \(.value.source.repo)"' "$settings"
} | while read -r name repo; do
  grep -q "> $name\$" <<<"$known" && continue
  claude plugin marketplace add "$repo" >/dev/null 2>&1 \
    || echo "warning: could not add plugin marketplace $name ($repo)" >&2
done

jq -r '.enabledPlugins // {} | to_entries[] | select(.value == true) | .key' "$settings" \
  | while read -r plugin; do
    grep -q "> $plugin\$" <<<"$installed" && continue
    claude plugin install "$plugin" >/dev/null 2>&1 \
      || echo "warning: could not install plugin $plugin" >&2
  done
exit 0
