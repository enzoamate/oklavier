{{- define "oklavier-agent.namespace" -}}
{{ .Values.agent.namespace | default "oklavier-agent" }}
{{- end -}}

{{- define "oklavier-agent.labels" -}}
app: oklavier-agent
app.kubernetes.io/managed-by: helm
app.kubernetes.io/part-of: oklavier
{{- end -}}
