#!/usr/bin/env bash
# Runs a scanner as an advisory check: its findings are shown (the job summary and a warning
# annotation) but never fail the job, while a scanner that cannot run at all still does, so a broken
# tool is noticed rather than reading as a clean result (decision 33).
#
# usage: advisory.sh <name> <findings-exit-code> <command> [args...]
set -uo pipefail

name=$1
findings=$2
shift 2

out="$(mktemp)"
"$@" >"$out" 2>&1
code=$?

cat "$out"
{
  echo "### $name"
  echo '```'
  tail -n 100 "$out"
  echo '```'
} >>"${GITHUB_STEP_SUMMARY:-/dev/null}"

if [ "$code" -eq 0 ]; then
  exit 0
fi
if [ "$code" -eq "$findings" ]; then
  echo "::warning title=$name::findings reported (advisory); see the job summary"
  exit 0
fi
echo "::error title=$name::the scanner failed to run (exit $code)"
exit "$code"
