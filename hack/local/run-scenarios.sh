#!/usr/bin/env bash
# Run all Plan 5-4 local E2E scenarios in order (operator must be running).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

"$ROOT/hack/local/scenario-service-patch.sh"
"$ROOT/hack/local/scenario-ingress-patch.sh"
"$ROOT/hack/local/scenario-auth-patch.sh"

echo ""
echo "========================================"
echo "All local E2E scenarios passed."
echo "========================================"
