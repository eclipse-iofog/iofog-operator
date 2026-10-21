#!/usr/bin/env bash
# Build a flavor-specific Helm chart tarball for GitHub Release and gh-pages.
# Usage: IMAGE_REGISTRY=ghcr.io/eclipse-iofog OPERATOR_CRD_GROUP=iofog.org hack/package-helm.sh <datasance|iofog> <version>
# Set LINT_ONLY=1 to stage the flavor chart and run helm lint without packaging.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FLAVOR="${1:-}"
VERSION="${2:-}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-}"
OPERATOR_CRD_GROUP="${OPERATOR_CRD_GROUP:-}"
LINT_ONLY="${LINT_ONLY:-}"
NATS_IMAGE_TAG="${NATS_IMAGE_TAG:-2.15.1-1}"

if [[ -z "$FLAVOR" || -z "$VERSION" ]]; then
	echo "usage: IMAGE_REGISTRY=... OPERATOR_CRD_GROUP=... $0 <datasance|iofog> <version>" >&2
	exit 1
fi

case "$FLAVOR" in
datasance)
	GROUP="${OPERATOR_CRD_GROUP:-datasance.com}"
	CRD_FILE="datasance.com_controlplanes.yaml"
	IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/datasance}"
	CONTROL_PLANE_NAME="pot"
	;;
iofog)
	GROUP="${OPERATOR_CRD_GROUP:-iofog.org}"
	CRD_FILE="iofog.org_controlplanes.yaml"
	IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/eclipse-iofog}"
	CONTROL_PLANE_NAME="iofog"
	;;
*)
	echo "FLAVOR must be datasance or iofog" >&2
	exit 1
	;;
esac

CHART_DIR="charts/iofog-operator"
CRD_SRC="config/crd/bases/${CRD_FILE}"
ARCHIVE="dist/iofog-operator-${VERSION}.tgz"
OVERRIDE="$(mktemp)"
STAGE="$(mktemp -d)"

command -v helm >/dev/null || {
	echo "helm is required" >&2
	exit 1
}
command -v python3 >/dev/null || {
	echo "python3 is required" >&2
	exit 1
}

cleanup() {
	rm -f "$OVERRIDE"
	rm -rf "$STAGE"
}
trap cleanup EXIT

OPERATOR_IMG="${IMAGE_REGISTRY}/operator:${VERSION}"
CONTROLLER_IMG="${IMAGE_REGISTRY}/controller:${VERSION}"
ROUTER_IMG="${IMAGE_REGISTRY}/router:${VERSION}"
NATS_IMG="${IMAGE_REGISTRY}/nats:${NATS_IMAGE_TAG}"

cp -a "$CHART_DIR" "${STAGE}/iofog-operator"
rm -f "${STAGE}/iofog-operator/datasance-values.yaml" "${STAGE}/iofog-operator/values.schema.json"
cp "$CRD_SRC" "${STAGE}/iofog-operator/crds/controlplanes.yaml"

cat >"${STAGE}/iofog-operator/templates/_flavor.tpl" <<EOF
{{/*
Flavor defaults written by hack/package-helm.sh for ${FLAVOR}.
*/}}
{{- define "iofog-operator.crdGroup" -}}${GROUP}{{- end }}
{{- define "iofog-operator.controlPlaneName" -}}${CONTROL_PLANE_NAME}{{- end }}
{{- define "iofog-operator.operatorImageDefault" -}}${OPERATOR_IMG}{{- end }}
EOF

STAGE="${STAGE}/iofog-operator" \
	OPERATOR_IMG="$OPERATOR_IMG" \
	CONTROLLER_IMG="$CONTROLLER_IMG" \
	ROUTER_IMG="$ROUTER_IMG" \
	NATS_IMG="$NATS_IMG" \
	python3 - <<'PY'
import os
import pathlib
import re

root = pathlib.Path(os.environ["STAGE"])
replacements = (
    ("operator", os.environ["OPERATOR_IMG"]),
    ("controller", os.environ["CONTROLLER_IMG"]),
    ("router", os.environ["ROUTER_IMG"]),
    ("nats", os.environ["NATS_IMG"]),
)
for name in ("values.yaml", "marketplace-values.yaml"):
    path = root / name
    text = path.read_text()
    for kind, image in replacements:
        text = re.sub(
            rf"ghcr.io/datasance/{kind}:[^\"'\s]+",
            image,
            text,
        )
    path.write_text(text)
PY

# Lint-only password. It is not merged into the packaged values.
cat >"$OVERRIDE" <<'EOF'
auth:
  bootstrap:
    password: ReplaceMe1!x
EOF

helm lint "${STAGE}/iofog-operator" -f "$OVERRIDE"

if [[ "$LINT_ONLY" == "1" ]]; then
	echo "Lint passed for FLAVOR=${FLAVOR} GROUP=${GROUP} CONTROL_PLANE_NAME=${CONTROL_PLANE_NAME}"
	exit 0
fi

mkdir -p dist
rm -f "$ARCHIVE"
helm package "${STAGE}/iofog-operator" \
	--version "${VERSION}" \
	--app-version "${VERSION}" \
	-d dist/

echo "Created ${ARCHIVE}"
