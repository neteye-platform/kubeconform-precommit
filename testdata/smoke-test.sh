#!/usr/bin/env bash
# Run the public kubeconform hook from this checkout against fixed fixtures.
# Usage: testdata/smoke-test.sh <hook manager command...>
# Example: testdata/smoke-test.sh uvx --from pre-commit==4.6.2 pre-commit
set -euo pipefail

"$@" try-repo . kubeconform \
  --verbose \
  --files testdata/valid.yaml testdata/valid.json
