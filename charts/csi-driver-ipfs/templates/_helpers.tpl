{{/*
Expand the name of the chart.
*/}}
{{- define "csi-driver-ipfs.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "csi-driver-ipfs.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels for metadata (output is used under metadata: in resources).
*/}}
{{- define "csi-driver-ipfs.labels" -}}
labels:
  helm.sh/chart: {{ include "csi-driver-ipfs.chart" . }}
  app.kubernetes.io/name: {{ include "csi-driver-ipfs.name" . }}
  app.kubernetes.io/instance: {{ .Release.Name }}
  app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
  app.kubernetes.io/managed-by: {{ .Release.Service }}
  {{- with .Values.customLabels }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
{{- end }}

{{/*
Selector labels for controller.
*/}}
{{- define "csi-driver-ipfs.controllerSelectorLabels" -}}
app.kubernetes.io/name: {{ include "csi-driver-ipfs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
Selector labels for node.
*/}}
{{- define "csi-driver-ipfs.nodeSelectorLabels" -}}
app.kubernetes.io/name: {{ include "csi-driver-ipfs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: node
{{- end }}
