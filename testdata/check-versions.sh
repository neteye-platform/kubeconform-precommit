#!/usr/bin/env bash
# Fail if intentionally duplicated tool versions drift apart.
set -euo pipefail

hooks=.pre-commit-hooks.yaml
workflow=.github/workflows/tests.yaml

# Usage: extract <description> <expected count> <file> <sed expression>
# Prints the single distinct value, or fails if the number of declarations is
# wrong or they disagree with each other.
extract() {
  local found
  found=$(sed -n "$4" "$3")
  if [[ $(grep -c . <<<"$found" || true) -ne $2 ]]; then
    echo "version check: expected $2 $1 declaration(s) in $3, found: ${found:-none}" >&2
    return 1
  fi
  if [[ $(sort -u <<<"$found" | wc -l) -ne 1 ]]; then
    echo "version check: $1 declarations in $3 differ: $(tr '\n' ' ' <<<"$found")" >&2
    return 1
  fi
  head -n 1 <<<"$found"
}

same() {
  if [[ $2 != "$3" ]]; then
    echo "version check: $1 differ: $2 vs $3" >&2
    return 1
  fi
}

go_toolchain=$(extract "go.mod toolchain" 1 go.mod 's/^toolchain go//p')
go_hooks=$(extract "hook language_version" 2 "$hooks" 's/^  language_version: "\(.*\)"$/\1/p')
kubeconform_lib=$(extract "go.mod kubeconform" 1 go.mod 's|^require github.com/yannh/kubeconform ||p')
kubeconform_hooks=$(extract "hook kubeconform" 2 "$hooks" 's|^    - github.com/yannh/kubeconform/cmd/kubeconform@||p')
extract "hook kustomize" 2 "$hooks" 's|^    - sigs.k8s.io/kustomize/kustomize/v5@||p' >/dev/null
min_hooks=$(extract "hook minimum_pre_commit_version" 2 "$hooks" 's/^  minimum_pre_commit_version: "\(.*\)"$/\1/p')
min_ci=$(extract "CI MIN_PRE_COMMIT_VERSION" 1 "$workflow" 's/^  MIN_PRE_COMMIT_VERSION: "\(.*\)"$/\1/p')

same "go.mod toolchain and hook language_version" "$go_toolchain" "$go_hooks"
same "go.mod kubeconform library and hook kubeconform binary" "$kubeconform_lib" "$kubeconform_hooks"
same "hook minimum_pre_commit_version and CI MIN_PRE_COMMIT_VERSION" "$min_hooks" "$min_ci"
