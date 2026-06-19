{{- define "iofog-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "iofog-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end -}}
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
{{- if .Values.operator.serviceAccount.create }}
{{- default (include "iofog-operator.fullname" .) .Values.operator.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.operator.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "iofog-operator.crdGroup" -}}
{{- .Values.crdGroup | default "datasance.com" -}}
{{- end }}

{{- define "iofog-operator.imageRegistry" -}}
{{- .Values.imageRegistry | default "ghcr.io/datasance" -}}
{{- end }}

{{- define "iofog-operator.operatorImage" -}}
{{- printf "%s/operator:%s" (include "iofog-operator.imageRegistry" .) .Chart.AppVersion -}}
{{- end }}

