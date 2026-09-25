#!/usr/bin/env bash
# Run both public hooks from this checkout's HEAD commit, as a consumer would.
# Usage: testdata/smoke-test.sh <hook manager command...>
# Example: testdata/smoke-test.sh uvx --from pre-commit==4.6.2 pre-commit
# Only committed changes are tested: the hook manager clones HEAD.
set -euo pipefail

config="$(mktemp)"
trap 'rm -f "$config"' EXIT

# The repository path must be absolute: hook managers clone it from their own
# cache directory. Both hooks share one managed Go environment.
cat > "$config" <<EOF
repos:
  - repo: $PWD
    rev: $(git rev-parse HEAD)
    hooks:
      - id: kubeconform
        files: ^testdata/(valid|invalid)\.(yaml|json)$
      - id: kubeconform-kustomize
        args: [testdata/kustomize, --, -strict]
EOF

run() {
  "$@" run --config "$config" --verbose "${hook_args[@]}"
}

hook_args=(kubeconform --files testdata/valid.yaml testdata/valid.json)
run "$@"

hook_args=(kubeconform-kustomize --files testdata/kustomize/kustomization.yaml)
run "$@"

# The invalid manifest must fail because kubeconform rejected it, not because
# of an unrelated setup or network error.
hook_args=(kubeconform --files testdata/invalid.yaml)
if output=$(run "$@" 2>&1); then
  echo "smoke test: expected testdata/invalid.yaml to fail validation" >&2
  echo "$output" >&2
  exit 1
fi
if [[ $output != *"testdata/invalid.yaml - ConfigMap smoke-test is invalid"* ]]; then
  echo "smoke test: testdata/invalid.yaml failed for an unexpected reason:" >&2
  echo "$output" >&2
  exit 1
fi
