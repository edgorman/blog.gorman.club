#!/bin/bash
set -euo pipefail

# Installs the exact toolchain versions this repo's CI pins - buf, terraform, and the moon/proto
# binaries themselves - so a cloud session can run `buf generate` and `terraform plan` against the
# same binaries CI uses, instead of whatever (if anything) happens to be preinstalled.
# See "Building with Pants" in .github/AGENTS.md and "Contract Layer" in packages/protos/AGENTS.md for where each version comes
# from. The Pants launcher is no longer installed here - see the moon/proto block below for why,
# and .github/AGENTS.md for what pants.toml/get-pants.sh are still needed for until #200.
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
# `required_version`, and pants.toml's [download-terraform] (whose known_versions block below
# quotes the same checksum). Needed for `terraform plan`/`apply` and for Pants' own Terraform
# backend to invoke the identical binary.
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
# Every proto/moon plugin - every toolchain (go, node, npm) *and* every third-party TOML plugin
# (buf, terraform, see .prototools) - is fetched as an OCI blob from ghcr.io, by way of
# pkg-containers.githubusercontent.com. That host isn't in the default allowlist either (confirmed:
# `moon query projects` fails with plugin::loader::registry::load_failure loading
# ghcr.io/moonrepo/go_toolchain, and `proto install buf` fails the same way loading
# ghcr.io/moonrepo/schema_tool - the generic loader every TOML plugin needs too, not just the
# builtin toolchains). So `moon --version` works here, but `moon setup`/`moon run`/`moon ci` and
# `proto install` do not, until ghcr.io joins the allowlist - a wider version of the
# registry.terraform.io gap below. buf and Terraform stay hand-installed above for that reason;
# see AGENTS.md's Commands section for what does and doesn't work in a cloud session.
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
