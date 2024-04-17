package template

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	console "github.com/pluralsh/console-client-go"
)

func renderTpl(input []byte, svc *console.GetServiceDeploymentForAgent_ServiceDeployment) ([]byte, error) {
	bindings := map[string]interface{}{
		"Configuration": configMap(svc),
		"Cluster":       clusterConfiguration(svc.Cluster),
		"Contexts":      contexts(svc),
	}

	tpl, err := template.New("gotpl").Funcs(sprig.TxtFuncMap()).Parse(string(input))
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	err = tpl.Execute(&buffer, bindings)
	return buffer.Bytes(), err
}
