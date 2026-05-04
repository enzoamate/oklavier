{{- define "oklavier.namespace" -}}
{{ .Values.global.namespace | default "oklavier" }}
{{- end -}}

{{- define "oklavier.labels" -}}
app.kubernetes.io/managed-by: helm
app.kubernetes.io/part-of: oklavier
{{- end -}}
