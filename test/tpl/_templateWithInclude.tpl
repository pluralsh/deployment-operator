# test/tpl/_templateWithInclude.tpl
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Configuration.name }}-main
data:
  included:
{{ include "_includedTemplate.tpl" . | indent 4 }}
