# test/tpl/_includedTemplate.tpl
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Configuration.name }}-included
data:
  version: {{ .Configuration.version }}
