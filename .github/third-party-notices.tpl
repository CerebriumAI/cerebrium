The Cerebrium CLI includes the following third-party software.
{{ range . }}
================================================================================
{{ .Name }} {{ .Version }}
License: {{ .LicenseName }} ({{ .LicenseURL }})
================================================================================

{{ if ne .LicensePath "Unknown" }}{{ .LicenseText }}{{ else }}This module does not publish a license file.{{ end }}
{{ end }}
