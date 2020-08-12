List of supported models:
{{ range .models -}}
* {{.}}
{{ end }}
{{- $printRaw:=.printRaw -}}
{{ if $printRaw }}
Raw data input:
{{ printf "%s" . }}
{{ end }}
