{{/*
Expand the name of the chart.
*/}}
{{- define "omnitun.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "omnitun.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "omnitun.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "omnitun.labels" -}}
helm.sh/chart: {{ include "omnitun.chart" . }}
{{ include "omnitun.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: omnitun
{{- end }}

{{/*
Selector labels
*/}}
{{- define "omnitun.selectorLabels" -}}
app.kubernetes.io/name: {{ include "omnitun.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Relay selector labels
*/}}
{{- define "omnitun.relaySelectorLabels" -}}
app.kubernetes.io/name: {{ include "omnitun.name" . }}-relay
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Relay labels
*/}}
{{- define "omnitun.relayLabels" -}}
helm.sh/chart: {{ include "omnitun.chart" . }}
{{ include "omnitun.relaySelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: omnitun
{{- end }}

{{/*
PostgreSQL service name
*/}}
{{- define "omnitun.postgresql.fullname" -}}
{{- if .Values.postgresql.fullnameOverride }}
{{- .Values.postgresql.fullnameOverride }}
{{- else }}
{{- printf "%s-postgresql" .Release.Name }}
{{- end }}
{{- end }}

{{/*
Valkey service name
*/}}
{{- define "omnitun.valkey.fullname" -}}
{{- if .Values.valkey.fullnameOverride }}
{{- .Values.valkey.fullnameOverride }}
{{- else }}
{{- printf "%s-valkey" .Release.Name }}
{{- end }}
{{- end }}

{{/*
NATS service name
*/}}
{{- define "omnitun.nats.fullname" -}}
{{- if .Values.nats.fullnameOverride }}
{{- .Values.nats.fullnameOverride }}
{{- else }}
{{- printf "%s-nats" .Release.Name }}
{{- end }}
{{- end }}

{{/*
ClickHouse service name
*/}}
{{- define "omnitun.clickhouse.fullname" -}}
{{- if .Values.clickhouse.fullnameOverride }}
{{- .Values.clickhouse.fullnameOverride }}
{{- else }}
{{- printf "%s-clickhouse" .Release.Name }}
{{- end }}
{{- end }}
