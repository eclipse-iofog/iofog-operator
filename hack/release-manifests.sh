#!/usr/bin/env bash
# Build a flavor-specific manifest tarball for GitHub Release.
# Usage: IMAGE_REGISTRY=ghcr.io/eclipse-iofog hack/release-manifests.sh <datasance|iofog> <version>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FLAVOR="${1:-}"
VERSION="${2:-}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/eclipse-iofog}"

if [[ -z "$FLAVOR" || -z "$VERSION" ]]; then
	echo "usage: IMAGE_REGISTRY=... $0 <datasance|iofog> <version>" >&2
	exit 1
fi

case "$FLAVOR" in
datasance)
	GROUP="datasance.com"
	CRD_FILE="datasance.com_controlplanes.yaml"
	;;
iofog)
	GROUP="iofog.org"
	CRD_FILE="iofog.org_controlplanes.yaml"
	;;
*)
	echo "FLAVOR must be datasance or iofog" >&2
	exit 1
	;;
esac

command -v kustomize >/dev/null || { echo "kustomize is required" >&2; exit 1; }

IMG="${IMAGE_REGISTRY}/operator:${VERSION}"
STAGE="dist/manifests-${FLAVOR}-${VERSION}"
ARCHIVE="dist/manifests-${FLAVOR}-${VERSION}.tar.gz"

mkdir -p dist
rm -rf "$STAGE" "$ARCHIVE"
mkdir -p "${STAGE}/crds" "${STAGE}/operator" "${STAGE}/samples"

cp "config/crd/bases/${CRD_FILE}" "${STAGE}/crds/"
sed "s|apiVersion: datasance.com/v3|apiVersion: ${GROUP}/v3|g" \
	config/cr/controlplane.yaml >"${STAGE}/samples/controlplane.yaml"

restore_kustomization() {
	git checkout -- config/operator/kustomization.yaml 2>/dev/null || true
}
trap restore_kustomization EXIT

(
	cd config/operator
	kustomize edit set image "ghcr.io/datasance/operator=${IMG}"
)
kustomize build config/default >"${STAGE}/operator/install.yaml"

tar -czf "$ARCHIVE" -C dist "$(basename "$STAGE")"
echo "Created ${ARCHIVE}"
