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

hook_args=(kubeconform --files testdata/invalid.yaml)
if run "$@"; then
  echo "smoke test: expected testdata/invalid.yaml to fail validation" >&2
  exit 1
fi
