#!/bin/bash
set -euo pipefail

# Installs the exact toolchain versions this repo's CI pins - terraform, and the moon/proto
# binaries themselves - so a cloud session can run `terraform plan` and `moon ci` against the
# same binaries CI uses, instead of whatever (if anything) happens to be preinstalled.
# See "Building with moon" in .github/AGENTS.md for where each version comes from.
#
# Local sessions skip this entirely: contributors manage their own toolchain versions, and this
# only exists to fill gaps in the cloud session base image (see
# https://code.claude.com/docs/en/cloud-environments#installed-tools).
if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

cd "$CLAUDE_PROJECT_DIR"

BIN_DIR="$HOME/.local/bin"
mkdir -p "$BIN_DIR"
echo "export PATH=\"$BIN_DIR:\$PATH\"" >>"$CLAUDE_ENV_FILE"
export PATH="$BIN_DIR:$PATH"

# --- terraform ---------------------------------------------------------------------------------
# The version .prototools pins, which setup-terraform in CI reads too and every providers.tf's
# `required_version` matches. Needed for `terraform plan`/`validate`. Checked against the release's
# own SHA256SUMS file, so there is no checksum here to go stale when the pin moves.
tf_version="$(sed -n 's/^terraform = "\([^"]*\)".*/\1/p' .prototools | head -1)"
if ! "$BIN_DIR/terraform" version 2>/dev/null | head -1 | grep -qx "Terraform v$tf_version"; then
  tf_tmp="$(mktemp -d)"
  trap 'rm -rf "$tf_tmp"' RETURN
  tf_zip="terraform_${tf_version}_linux_amd64.zip"
  curl -fsSL -o "$tf_tmp/$tf_zip" "https://releases.hashicorp.com/terraform/$tf_version/$tf_zip"
  curl -fsSL -o "$tf_tmp/SHA256SUMS" "https://releases.hashicorp.com/terraform/$tf_version/terraform_${tf_version}_SHA256SUMS"
  (cd "$tf_tmp" && grep " $tf_zip\$" SHA256SUMS | sha256sum -c -)
  unzip -q "$tf_tmp/$tf_zip" -d "$tf_tmp"
  mv "$tf_tmp/terraform" "$BIN_DIR/terraform"
  rm -rf "$tf_tmp"
  trap - RETURN
fi

# --- proto + moon -----------------------------------------------------------------
# Pinned GitHub release binaries with verified checksums, the same pattern terraform above
# use - not `curl https://moonrepo.dev/install/moon.sh | bash`, since that installer isn't
# reachable through the session proxy (403), and moonrepo.dev isn't in the default allowlist.
#
# This gets the `moon`/`proto` binaries only, deliberately not `proto install`/`moon setup`.
# Every proto/moon plugin is fetched as an OCI blob from ghcr.io by default, by way of
# pkg-containers.githubusercontent.com, which isn't in the default allowlist (`moon query
# projects` fails with plugin::loader::registry::load_failure loading ghcr.io/moonrepo/go_toolchain).
# Two documented moon env vars (https://moonrepo.dev/docs/env-vars), exported below, route around
# that instead:
# - MOON_PLUGINS_USE_URL_DIST loads each plugin from its github.com/moonrepo/plugins release asset
#   (and sets PROTO_PLUGINS_USE_URL_DIST for proto's own plugins) rather than from ghcr.io.
# - MOON_TOOLCHAIN_FORCE_GLOBALS runs tasks with the go/node/npm already on PATH plus
#   Terraform from above, rather than having proto download them - dl.google.com, where proto
#   fetches Go from, is blocked too. The versions still match .prototools: go.mod's `go` line makes
#   GOTOOLCHAIN=auto fetch the pinned Go (see below), and the base image's Node 22/npm match.
# With both set, `moon run`/`moon ci` work here. See AGENTS.md's Commands section for the two
# tasks that still can't (Terraform `validate`, the backend image).
# renovate: datasource=github-releases depName=moonrepo/proto extractVersion=^v(?<version>.+)$
proto_version='0.62.3'
moon_version="$(sed -n 's/^moon = "\([^"]*\)".*/\1/p' .prototools | head -1)"
if ! "$BIN_DIR/proto" --version 2>/dev/null | grep -qx "$proto_version"; then
  proto_tmp="$(mktemp -d)"
  trap 'rm -rf "$proto_tmp"' RETURN
  proto_asset='proto_cli-x86_64-unknown-linux-gnu.tar.xz'
  curl -fsSL -o "$proto_tmp/$proto_asset" \
    "https://github.com/moonrepo/proto/releases/download/v$proto_version/$proto_asset"
  curl -fsSL -o "$proto_tmp/$proto_asset.sha256" \
    "https://github.com/moonrepo/proto/releases/download/v$proto_version/$proto_asset.sha256"
  (cd "$proto_tmp" && sha256sum -c "$proto_asset.sha256")
  tar -xJf "$proto_tmp/$proto_asset" -C "$proto_tmp"
  mv "$proto_tmp/proto_cli-x86_64-unknown-linux-gnu/proto" "$BIN_DIR/proto"
  chmod +x "$BIN_DIR/proto"
  rm -rf "$proto_tmp"
  trap - RETURN
fi

if ! "$BIN_DIR/moon" --version 2>/dev/null | grep -qx "$moon_version"; then
  moon_tmp="$(mktemp -d)"
  trap 'rm -rf "$moon_tmp"' RETURN
  moon_asset='moon_cli-x86_64-unknown-linux-gnu.tar.xz'
  curl -fsSL -o "$moon_tmp/$moon_asset" \
    "https://github.com/moonrepo/moon/releases/download/v$moon_version/$moon_asset"
  curl -fsSL -o "$moon_tmp/$moon_asset.sha256" \
    "https://github.com/moonrepo/moon/releases/download/v$moon_version/$moon_asset.sha256"
  (cd "$moon_tmp" && sha256sum -c "$moon_asset.sha256")
  tar -xJf "$moon_tmp/$moon_asset" -C "$moon_tmp"
  mv "$moon_tmp/moon_cli-x86_64-unknown-linux-gnu/moon" "$BIN_DIR/moon"
  chmod +x "$BIN_DIR/moon"
  rm -rf "$moon_tmp"
  trap - RETURN
fi

echo 'export MOON_PLUGINS_USE_URL_DIST=true' >>"$CLAUDE_ENV_FILE"
echo 'export MOON_TOOLCHAIN_FORCE_GLOBALS=true' >>"$CLAUDE_ENV_FILE"

# --- Go 1.26.0 toolchain -----------------------------------------------------------------------
# go.mod declares `go 1.26.0`; GOTOOLCHAIN defaults to "auto", so any `go` command run inside the
# module fetches and caches the matching toolchain from proxy.golang.org on its own. `go mod
# download` both triggers that and warms the module cache CI would otherwise populate per-run.
(cd services/backend && go mod download)

# --- npm dependencies --------------------------------------------------------------------------
# `npm ci` rather than `npm install`, matching CI (frontend-build, pull-request.yaml): it installs
# exactly what package-lock.json records and never rewrites it, so a session can't start with an
# unrelated lockfile diff that then rides along into its PR.
# The frontend's `npm ci` installs everything `tsc`/`vite`/`vitest` need; packages/api's installs
# openapi-typescript, which `moon run packages/api:generate` runs to write the frontend's types.
(cd services/frontend && npm ci)
(cd packages/api && npm ci)
