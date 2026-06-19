#!/usr/bin/env bash
# Generate CRD YAML for one mirror flavor (datasance.com or iofog.org).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SOURCE_GROUP="datasance.com"
FLAVOR="${1:-}"

case "$FLAVOR" in
datasance)
	TARGET_GROUP="datasance.com"
	;;
iofog)
	TARGET_GROUP="iofog.org"
	;;
*)
	echo "usage: $0 datasance|iofog" >&2
	exit 1
	;;
esac

GV_FILES=(
	apis/controlplanes/v3/groupversion_info.go
)

patch_group() {
	local from="$1"
	local to="$2"
	local f
	for f in "${GV_FILES[@]}"; do
		sed "s/${from}/${to}/g" "$f" >"${f}.gen"
		mv "${f}.gen" "$f"
	done
}

if [[ "$TARGET_GROUP" != "$SOURCE_GROUP" ]]; then
	patch_group "$SOURCE_GROUP" "$TARGET_GROUP"
	trap 'patch_group "'"$TARGET_GROUP"'" "'"$SOURCE_GROUP"'"' EXIT
fi

controller-gen crd:crdVersions=v1,allowDangerousTypes=true \
	paths="./apis/controlplanes/..." \
	output:crd:artifacts:config=config/crd/bases
