#!/usr/bin/env bash
# Generate a flavor-specific OLM bundle and validate it.
# Usage: IMAGE_REGISTRY=ghcr.io/datasance hack/gen-bundle.sh <datasance|iofog> <version>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FLAVOR="${1:-}"
VERSION="${2:-}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/eclipse-iofog}"
BUNDLE_PACKAGE="${BUNDLE_PACKAGE:-iofog-operator}"
BUNDLE_CHANNEL="${BUNDLE_CHANNEL:-stable}"

if [[ -z "$FLAVOR" || -z "$VERSION" ]]; then
	echo "usage: IMAGE_REGISTRY=... $0 <datasance|iofog> <version>" >&2
	exit 1
fi

case "$FLAVOR" in
datasance | iofog) ;;
*)
	echo "FLAVOR must be datasance or iofog" >&2
	exit 1
	;;
esac

command -v kustomize >/dev/null || {
	echo "kustomize is required" >&2
	exit 1
}
command -v operator-sdk >/dev/null || {
	echo "operator-sdk is required" >&2
	exit 1
}

IMG="${IMAGE_REGISTRY}/operator:${VERSION}"
METADATA_OPTS=(--package "${BUNDLE_PACKAGE}" --channels "${BUNDLE_CHANNEL}" --default-channel "${BUNDLE_CHANNEL}")

restore_kustomization() {
	git checkout -- config/operator/kustomization.yaml 2>/dev/null || true
}
trap restore_kustomization EXIT

make manifests-flavor FLAVOR="$FLAVOR"

operator-sdk generate kustomize manifests -q

(
	cd config/operator
	kustomize edit set image "ghcr.io/datasance/operator=${IMG}"
)

rm -rf bundle/manifests bundle/metadata

kustomize build --load-restrictor LoadRestrictionsNone "config/manifests/overlays/${FLAVOR}" |
	operator-sdk generate bundle -q --overwrite --version "$VERSION" "${METADATA_OPTS[@]}"

operator-sdk bundle validate ./bundle
echo "Bundle generated for FLAVOR=${FLAVOR} VERSION=${VERSION}"
