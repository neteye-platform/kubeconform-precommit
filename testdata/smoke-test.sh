#!/usr/bin/env bash
# Run both public hooks from this checkout's HEAD commit, as a consumer would.
# Usage: testdata/smoke-test.sh <hook manager command...>
# Example: testdata/smoke-test.sh uvx --from pre-commit==4.6.2 pre-commit
# Only committed changes are tested: the hook manager clones HEAD.
set -euo pipefail

config="$(mktemp)"
invalid_overlay_config="$(mktemp)"
trap 'rm -f "$config" "$invalid_overlay_config"' EXIT

repo_path="$(git rev-parse --show-toplevel)"
repo_url="file://${repo_path}"

# The local file URL causes the hook manager to clone the repository from its
# cache directory. Both hooks share one managed Go environment.
cat > "$config" <<EOF
repos:
  - repo: "$repo_url"
    rev: $(git rev-parse HEAD)
    hooks:
      - id: kubeconform
        files: ^testdata/(valid|invalid)\.(yaml|json)$
      - id: kubeconform-kustomize
        args: ["testdata/kustomize*", --, -strict]
EOF

# A second generated config exercises kubeconform-kustomize's own failure
# path: a valid Kustomize overlay that builds successfully but renders a
# manifest kubeconform rejects. Kept separate so the positive case above
# keeps its fixed args untouched.
cat > "$invalid_overlay_config" <<EOF
repos:
  - repo: "$repo_url"
    rev: $(git rev-parse HEAD)
    hooks:
      - id: kubeconform-kustomize
        args: ["testdata/invalid-overlay", --, -strict]
EOF

run() {
  "$@" run --config "$config_file" --verbose "${hook_args[@]}"
}

config_file="$config"

hook_args=(kubeconform --files testdata/valid.yaml --files testdata/valid.json)
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

# The invalid overlay must fail because kubeconform-kustomize's own
# kubeconform validation step rejected the rendered manifest, not because
# kustomize build or hook setup failed.
config_file="$invalid_overlay_config"
hook_args=(kubeconform-kustomize --files testdata/invalid-overlay/kustomization.yaml)
if output=$(run "$@" 2>&1); then
  echo "smoke test: expected testdata/invalid-overlay to fail validation" >&2
  echo "$output" >&2
  exit 1
fi
if [[ $output != *"stdin - ConfigMap smoke-test is invalid"* ]]; then
  echo "smoke test: testdata/invalid-overlay failed for an unexpected reason:" >&2
  echo "$output" >&2
  exit 1
fi
