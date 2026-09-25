#!/bin/bash
set -euo pipefail

# Installs the exact toolchain versions this repo's CI pins - buf, terraform, and the moon/proto
# binaries themselves - so a cloud session can run `buf generate` and `terraform plan` against the
# same binaries CI uses, instead of whatever (if anything) happens to be preinstalled.
# See "Building with moon" in .github/AGENTS.md and "Contract Layer" in packages/protos/AGENTS.md for where each version comes
# from.
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

# --- buf 1.47.2 ------------------------------------------------------------------------------
# The CLI version pull-request.yaml's protos-drift check actually installs, via
# `bufbuild/buf-setup-action`'s `version: '1.47.2'` input - not the action's own tag (v1.50.0),
# a different number. Needed for `buf generate`/`buf lint`/`buf format` in packages/protos.
if ! "$BIN_DIR/buf" --version 2>/dev/null | grep -qx '1.47.2'; then
  buf_tmp="$(mktemp -d)"
  trap 'rm -rf "$buf_tmp"' RETURN
  curl -fsSL -o "$buf_tmp/buf-Linux-x86_64" "https://github.com/bufbuild/buf/releases/download/v1.47.2/buf-Linux-x86_64"
  curl -fsSL -o "$buf_tmp/sha256.txt" "https://github.com/bufbuild/buf/releases/download/v1.47.2/sha256.txt"
  (cd "$buf_tmp" && grep -x '[0-9a-f]\{64\}  buf-Linux-x86_64' sha256.txt | sha256sum -c -)
  chmod +x "$buf_tmp/buf-Linux-x86_64"
  mv "$buf_tmp/buf-Linux-x86_64" "$BIN_DIR/buf"
  rm -rf "$buf_tmp"
  trap - RETURN
fi

# --- terraform 1.15.8 -------------------------------------------------------------------------
# Matches infrastructure/env/providers.tf and infrastructure/root/providers.tf's
# `required_version`, and .prototools' own terraform pin. Needed for `terraform plan`/`apply`.
if ! "$BIN_DIR/terraform" version 2>/dev/null | head -1 | grep -qx 'Terraform v1.15.8'; then
  tf_tmp="$(mktemp -d)"
  trap 'rm -rf "$tf_tmp"' RETURN
  curl -fsSL -o "$tf_tmp/terraform.zip" \
    "https://releases.hashicorp.com/terraform/1.15.8/terraform_1.15.8_linux_amd64.zip"
  echo "d25ce7b6902013ad905db3d2eab0be4cd905887fe88b81a6171b8d5503c31f3d  $tf_tmp/terraform.zip" | sha256sum -c -
  unzip -q "$tf_tmp/terraform.zip" -d "$tf_tmp"
  mv "$tf_tmp/terraform" "$BIN_DIR/terraform"
  rm -rf "$tf_tmp"
  trap - RETURN
fi

# --- proto 0.62.3 + moon 2.5.5 -----------------------------------------------------------------
# Pinned GitHub release binaries with verified checksums, the same pattern buf/terraform above
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
# - MOON_TOOLCHAIN_FORCE_GLOBALS runs tasks with the go/node/npm already on PATH plus buf and
#   Terraform from above, rather than having proto download them - dl.google.com, where proto
#   fetches Go from, is blocked too. The versions still match .prototools: go.mod's `go` line makes
#   GOTOOLCHAIN=auto fetch the pinned Go (see below), and the base image's Node 22/npm match.
# With both set, `moon run`/`moon ci` work here. See AGENTS.md's Commands section for the two
# tasks that still can't (Terraform `validate`, the backend image).
if ! "$BIN_DIR/proto" --version 2>/dev/null | grep -qx '0.62.3'; then
  proto_tmp="$(mktemp -d)"
  trap 'rm -rf "$proto_tmp"' RETURN
  proto_asset='proto_cli-x86_64-unknown-linux-gnu.tar.xz'
  curl -fsSL -o "$proto_tmp/$proto_asset" \
    "https://github.com/moonrepo/proto/releases/download/v0.62.3/$proto_asset"
  curl -fsSL -o "$proto_tmp/$proto_asset.sha256" \
    "https://github.com/moonrepo/proto/releases/download/v0.62.3/$proto_asset.sha256"
  (cd "$proto_tmp" && sha256sum -c "$proto_asset.sha256")
  tar -xJf "$proto_tmp/$proto_asset" -C "$proto_tmp"
  mv "$proto_tmp/proto_cli-x86_64-unknown-linux-gnu/proto" "$BIN_DIR/proto"
  chmod +x "$BIN_DIR/proto"
  rm -rf "$proto_tmp"
  trap - RETURN
fi

if ! "$BIN_DIR/moon" --version 2>/dev/null | grep -qx '2.5.5'; then
  moon_tmp="$(mktemp -d)"
  trap 'rm -rf "$moon_tmp"' RETURN
  moon_asset='moon_cli-x86_64-unknown-linux-gnu.tar.xz'
  curl -fsSL -o "$moon_tmp/$moon_asset" \
    "https://github.com/moonrepo/moon/releases/download/v2.5.5/$moon_asset"
  curl -fsSL -o "$moon_tmp/$moon_asset.sha256" \
    "https://github.com/moonrepo/moon/releases/download/v2.5.5/$moon_asset.sha256"
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
# Installs ts-proto (services/frontend/package.json devDependency), which packages/protos/buf.gen.yaml
# invokes from services/frontend/node_modules/.bin, plus everything `tsc`/`vite`/`jest` need.
(cd services/frontend && npm ci)
