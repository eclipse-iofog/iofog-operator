#!/usr/bin/env bash
# Render flavor-stamped Helm gh-pages landing docs from docs/helm/index.html.tpl.
# Usage: HELM_REPO_BASE_URL=... OCI_SOURCE_REPO=... hack/render-helm-docs.sh <datasance|iofog> <version> <output-dir>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FLAVOR="${1:-}"
VERSION="${2:-}"
OUTPUT_DIR="${3:-}"

if [[ -z "$FLAVOR" || -z "$VERSION" || -z "$OUTPUT_DIR" ]]; then
	echo "usage: HELM_REPO_BASE_URL=... OCI_SOURCE_REPO=... $0 <datasance|iofog> <version> <output-dir>" >&2
	exit 1
fi

case "$FLAVOR" in
datasance)
	export PRODUCT_NAME="Datasance PoT"
	export CRD_GROUP="${OPERATOR_CRD_GROUP:-datasance.com}"
	export IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/datasance}"
	export HELM_REPO_BASE_URL="${HELM_REPO_BASE_URL:-https://datasance.github.io/iofog-operator}"
	export OCI_SOURCE_REPO="${OCI_SOURCE_REPO:-https://github.com/Datasance/iofog-operator}"
	export SIBLING_PRODUCT="Eclipse ioFog"
	export SIBLING_HELM_URL="https://eclipse-iofog.github.io/iofog-operator"
	;;
iofog)
	export PRODUCT_NAME="Eclipse ioFog"
	export CRD_GROUP="${OPERATOR_CRD_GROUP:-iofog.org}"
	export IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/eclipse-iofog}"
	export HELM_REPO_BASE_URL="${HELM_REPO_BASE_URL:-https://eclipse-iofog.github.io/iofog-operator}"
	export OCI_SOURCE_REPO="${OCI_SOURCE_REPO:-https://github.com/eclipse-iofog/iofog-operator}"
	export SIBLING_PRODUCT="Datasance PoT"
	export SIBLING_HELM_URL="https://datasance.github.io/iofog-operator"
	;;
*)
	echo "FLAVOR must be datasance or iofog" >&2
	exit 1
	;;
esac

export VERSION
export CHART_README_URL="${OCI_SOURCE_REPO}/blob/develop/charts/iofog-operator/README.md"

TEMPLATE="${ROOT}/docs/helm/index.html.tpl"
if [[ ! -f "$TEMPLATE" ]]; then
	echo "missing template: ${TEMPLATE}" >&2
	exit 1
fi

command -v envsubst >/dev/null || {
	echo "envsubst is required" >&2
	exit 1
}

mkdir -p "$OUTPUT_DIR"
envsubst '${PRODUCT_NAME} ${CRD_GROUP} ${IMAGE_REGISTRY} ${HELM_REPO_BASE_URL} ${OCI_SOURCE_REPO} ${VERSION} ${SIBLING_PRODUCT} ${SIBLING_HELM_URL} ${CHART_README_URL}' \
	<"$TEMPLATE" >"${OUTPUT_DIR}/index.html"
touch "${OUTPUT_DIR}/.nojekyll"

echo "Rendered ${OUTPUT_DIR}/index.html for FLAVOR=${FLAVOR} VERSION=${VERSION}"
