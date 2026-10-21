{{- define "iofog-operator.name" -}}
iofog-operator
{{- end }}

{{- define "iofog-operator.fullname" -}}
iofog-operator
{{- end }}

{{- define "iofog-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "iofog-operator.labels" -}}
helm.sh/chart: {{ include "iofog-operator.chart" . }}
{{ include "iofog-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "iofog-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "iofog-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "iofog-operator.serviceAccountName" -}}
iofog-operator
{{- end }}

{{- define "iofog-operator.operatorImage" -}}
{{- .Values.images.operator | default (include "iofog-operator.operatorImageDefault" .) -}}
{{- end }}

{{/*
annotationMap renders annotations as a YAML map.
A map is written through. A string is one "key: value" line per annotation.
Blank lines are skipped. A duplicate key or a line without a colon fails the render.
*/}}
{{- define "iofog-operator.annotationMap" -}}
{{- if not . -}}
{{- else if kindIs "string" . -}}
{{- $out := dict -}}
{{- range splitList "\n" . -}}
{{- $line := . | replace "\r" "" | trim -}}
{{- if $line -}}
{{- if not (regexMatch "^[^:]+:" $line) -}}
{{- fail (printf "annotation %q must be key: value" $line) -}}
{{- end -}}
{{- $key := regexReplaceAll ":.*$" $line "" | trim -}}
{{- $value := regexReplaceAll "^[^:]+:" $line "" | trim -}}
{{- if not $key -}}
{{- fail (printf "annotation %q must be key: value" $line) -}}
{{- end -}}
{{- if hasKey $out $key -}}
{{- fail (printf "duplicate annotation key %q" $key) -}}
{{- end -}}
{{- $_ := set $out $key $value -}}
{{- end -}}
{{- end -}}
{{- if gt (len $out) 0 -}}
{{- toYaml $out -}}
{{- end -}}
{{- else if kindIs "map" . -}}
{{- $out := dict -}}
{{- range $k, $v := . -}}
{{- $key := $k | toString | trim -}}
{{- if not $key -}}
{{- fail "annotation keys must not be empty" -}}
{{- end -}}
{{- $_ := set $out $key ($v | toString) -}}
{{- end -}}
{{- if gt (len $out) 0 -}}
{{- toYaml $out -}}
{{- end -}}
{{- else -}}
{{- fail "annotations must be a map or one key: value per line" -}}
{{- end -}}
{{- end -}}

{{/*
trafficPolicy returns Local, Cluster, or an empty string.
auto and empty omit the field so the operator can apply its default.
*/}}
{{- define "iofog-operator.trafficPolicy" -}}
{{- $policy := . | default "auto" | toString -}}
{{- if or (eq $policy "auto") (eq $policy "") -}}
{{- else if or (eq $policy "Local") (eq $policy "Cluster") -}}
{{- $policy -}}
{{- else -}}
{{- fail (printf "externalTrafficPolicy must be auto, Local, or Cluster, got %q" $policy) -}}
{{- end -}}
{{- end -}}

{{/*
trustProxyValue returns true, false, or an empty string.
auto omits the field so an ingress can still default trust proxy on.
*/}}
{{- define "iofog-operator.trustProxyValue" -}}
{{- if kindIs "bool" . -}}
{{- if . -}}true{{- else -}}false{{- end -}}
{{- else -}}
{{- $s := . | default "auto" | toString -}}
{{- if or (eq $s "auto") (eq $s "") (eq $s "<nil>") -}}
{{- else if or (eq $s "on") (eq $s "true") -}}
true
{{- else if or (eq $s "off") (eq $s "false") -}}
false
{{- else -}}
{{- fail (printf "controller.trustProxy must be auto, on, or off, got %q" $s) -}}
{{- end -}}
{{- end -}}
{{- end -}}
