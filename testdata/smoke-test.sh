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
        files: ^testdata/valid\.(yaml|json)$
      - id: kubeconform-kustomize
        args: [testdata/kustomize, --, -strict]
EOF

"$@" run --config "$config" --verbose \
  --files testdata/valid.yaml testdata/valid.json
