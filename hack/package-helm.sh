#!/usr/bin/env bash
# Build a flavor-specific Helm chart tarball for GitHub Release and gh-pages.
# Usage: IMAGE_REGISTRY=ghcr.io/eclipse-iofog OPERATOR_CRD_GROUP=iofog.org hack/package-helm.sh <datasance|iofog> <version>
# Set LINT_ONLY=1 to stage the flavor CRD and run helm lint without packaging.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FLAVOR="${1:-}"
VERSION="${2:-}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-}"
OPERATOR_CRD_GROUP="${OPERATOR_CRD_GROUP:-}"
LINT_ONLY="${LINT_ONLY:-}"
NATS_IMAGE_TAG="${NATS_IMAGE_TAG:-2.14.3}"

if [[ -z "$FLAVOR" || -z "$VERSION" ]]; then
	echo "usage: IMAGE_REGISTRY=... OPERATOR_CRD_GROUP=... $0 <datasance|iofog> <version>" >&2
	exit 1
fi

case "$FLAVOR" in
datasance)
	GROUP="${OPERATOR_CRD_GROUP:-datasance.com}"
	CRD_FILE="datasance.com_controlplanes.yaml"
	IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/datasance}"
	;;
iofog)
	GROUP="${OPERATOR_CRD_GROUP:-iofog.org}"
	CRD_FILE="iofog.org_controlplanes.yaml"
	IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/eclipse-iofog}"
	;;
*)
	echo "FLAVOR must be datasance or iofog" >&2
	exit 1
	;;
esac

CHART_DIR="charts/iofog-operator"
CRD_DEST="${CHART_DIR}/crds/controlplanes.yaml"
CRD_SRC="config/crd/bases/${CRD_FILE}"
ARCHIVE="dist/iofog-operator-${VERSION}.tgz"
OVERRIDE="$(mktemp)"
STAGE=""

command -v helm >/dev/null || {
	echo "helm is required" >&2
	exit 1
}

cleanup() {
	if git ls-files --error-unmatch "$CRD_DEST" >/dev/null 2>&1; then
		git checkout -- "$CRD_DEST" 2>/dev/null || true
	else
		cp "config/crd/bases/datasance.com_controlplanes.yaml" "$CRD_DEST"
	fi
	rm -f "$OVERRIDE"
	if [[ -n "$STAGE" && -d "$STAGE" ]]; then
		rm -rf "$STAGE"
	fi
}
trap cleanup EXIT

mkdir -p dist
cp "$CRD_SRC" "$CRD_DEST"

OPERATOR_IMG="${IMAGE_REGISTRY}/operator:${VERSION}"
CONTROLLER_IMG="${IMAGE_REGISTRY}/controller:${VERSION}"
ROUTER_IMG="${IMAGE_REGISTRY}/router:${VERSION}"
NATS_IMG="${IMAGE_REGISTRY}/nats:${NATS_IMAGE_TAG}"

cat >"$OVERRIDE" <<EOF
crdGroup: ${GROUP}
imageRegistry: ${IMAGE_REGISTRY}
operator:
  image: ${OPERATOR_IMG}
controlplane:
  spec:
    auth:
      bootstrap:
        password: ReplaceMe1!
    images:
      controller: "${CONTROLLER_IMG}"
      router: "${ROUTER_IMG}"
      nats: "${NATS_IMG}"
EOF

helm lint "$CHART_DIR" -f "$OVERRIDE"

if [[ "$LINT_ONLY" == "1" ]]; then
	echo "Lint passed for FLAVOR=${FLAVOR} GROUP=${GROUP}"
	exit 0
fi

STAGE="$(mktemp -d)"
cp -r "$CHART_DIR" "${STAGE}/iofog-operator"
cp "$CRD_SRC" "${STAGE}/iofog-operator/crds/controlplanes.yaml"

ruby -ryaml -e '
def deep_merge(base, override)
  override.each do |key, value|
    if value.is_a?(Hash) && base[key].is_a?(Hash)
      deep_merge(base[key], value)
    else
      base[key] = value
    end
  end
  base
end

chart_values = ARGV[0]
override_values = ARGV[1]
base = YAML.load_file(chart_values)
over = YAML.load_file(override_values)
File.write(chart_values, deep_merge(base, over).to_yaml)
' "${STAGE}/iofog-operator/values.yaml" "$OVERRIDE"

rm -f "$ARCHIVE"
helm package "${STAGE}/iofog-operator" \
	--version "${VERSION}" \
	--app-version "${VERSION}" \
	-d dist/

echo "Created ${ARCHIVE}"
