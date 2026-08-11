{{/*
Expand the name of the chart.
*/}}
{{- define "gomessenger.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gomessenger.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gomessenger.labels" -}}
helm.sh/chart: {{ include "gomessenger.chart" . }}
{{ include "gomessenger.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "gomessenger.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gomessenger.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gomessenger.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "gomessenger.serviceLabels" -}}
{{ include "gomessenger.selectorLabels" . }}
app.kubernetes.io/component: microservice
{{- end }}

{{- define "gomessenger.microserviceLabels" -}}
{{ include "gomessenger.selectorLabels" .root }}
app.kubernetes.io/component: {{ .name }}
{{- end }}

{{- define "gomessenger.image" -}}
{{- $image := .svc.image -}}
{{- $tag := .global.imageTag -}}
{{- if .global.imageRegistry -}}
{{- printf "%s/%s:%s" .global.imageRegistry $image $tag -}}
{{- else -}}
{{- printf "%s:%s" $image $tag -}}
{{- end -}}
{{- end }}
