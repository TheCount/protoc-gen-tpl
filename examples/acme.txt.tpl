List of supported models:
{{ range .models -}}
* {{.}}
{{ end }}
Raw data input:
{{ printf "%s" . }}
