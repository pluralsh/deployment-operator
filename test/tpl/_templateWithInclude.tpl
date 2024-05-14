# test/tpl/_templateWithInclude.tpl
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Configuration.name }}-main
data:
  version: {{ .Configuration.version | quote}}
  included: |
    {{ include "_includedTemplate.tpl" . }}
