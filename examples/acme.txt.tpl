List of supported models:
{{ range .models -}}
* {{.}}
{{- end }}
{{ printf "%s" . }}
