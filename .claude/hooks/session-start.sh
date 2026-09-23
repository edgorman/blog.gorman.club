#!/bin/bash
set -euo pipefail

# Installs the exact toolchain versions this repo's CI pins - buf and terraform, plus a
# best-effort Pants launcher - so a cloud session can run `buf generate` and `terraform plan`
# against the same binaries CI uses, instead of whatever (if anything) happens to be preinstalled.
# See "Building with Pants" in .github/AGENTS.md and "Contract Layer" in packages/protos/AGENTS.md for where each version comes
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

# --- Pants launcher (best-effort) --------------------------------------------------------------
# pants.toml pins `pants_version = "2.33.0"`; getting `pants` here means running the checked-in
# get-pants.sh (see .github/AGENTS.md's "Building with Pants" section for why it's vendored at the repo
# root), which installs the scie-pants launcher - that in turn reads pants.toml itself and
# resolves 2.33.0 on first invocation. Using the vendored copy rather than curling
# static.pantsbuild.org directly is Pants' own recommendation, and also sidesteps that host not
# being in a cloud session's default network allowlist - get-pants.sh's own downloads go to
# github.com/pantsbuild/scie-pants, which is.
#
# This is genuinely optional: .github/AGENTS.md's "Building with Pants" section is explicit that neither
# service uses Pants as a local dev wrapper - only CI does, via `pants --changed-since=origin/main
# lint check` / `test package`. Day-to-day work in a session runs `go test`, `npm test`,
# `buf generate`, `terraform plan`, etc. directly, same as AGENTS.md tells a contributor to. `pants`
# is only worth having on hand to reproduce a Pants-specific CI failure (a `tailor --check` gap, a
# BUILD-graph issue) locally, so a failure to install it here shouldn't fail the whole hook.
if ! command -v pants >/dev/null 2>&1; then
  if ! ./get-pants.sh; then
    echo "warning: could not install the pants launcher." \
      "Not fatal - see the comment above this step in $0." >&2
  fi
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
