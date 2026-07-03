#!/usr/bin/env bash
# demo.sh — runs tfx against all four fixture plans and prints output.
# Use this to record a terminal demo:
#   asciinema rec demo.cast --command ./demo.sh
#   script -q demo.txt ./demo.sh

set -euo pipefail

TFX="${1:-./tfx}"

# Auto-build if the binary doesn't exist
if [ ! -x "$TFX" ]; then
  echo "Building tfx..."
  go build -o "${TFX}" ./cli/
fi

hr() { printf '\n%s\n' "$(printf '=%.0s' {1..60})"; }
run() {
  local label="$1"; shift
  hr
  printf '$ tfx analyze %s\n\n' "$label"
  sleep 0.5
  "$TFX" analyze "$@" || true
  sleep 1
}

printf 'tfx — Terraform plan cost & risk analyzer demo\n'
sleep 1

run "fixtures/clean_plan.json"              fixtures/clean_plan.json
run "fixtures/cost_increase_plan.json"      fixtures/cost_increase_plan.json
run "fixtures/destructive_plan.json"        fixtures/destructive_plan.json
run "fixtures/security_misconfig_plan.json" fixtures/security_misconfig_plan.json
run "fixtures/destructive_plan.json --format json" fixtures/destructive_plan.json --format json

hr
printf '\nDemo complete.\n'
