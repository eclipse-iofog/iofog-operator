{{/*
Flavor defaults for a local Datasance build.
hack/package-helm.sh rewrites this file in the packaged chart.
*/}}
{{- define "iofog-operator.crdGroup" -}}datasance.com{{- end }}
{{- define "iofog-operator.controlPlaneName" -}}pot{{- end }}
{{- define "iofog-operator.operatorImageDefault" -}}ghcr.io/datasance/operator:3.9.3{{- end }}
