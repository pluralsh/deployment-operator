package template

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/template"
)

func renderTpl(input []byte, svc *console.ServiceDeploymentForAgent) ([]byte, error) {
	bindings := map[string]interface{}{
		"Configuration": configMap(svc),
		"Cluster":       clusterConfiguration(svc.Cluster),
		"Contexts":      contexts(svc),
		"Imports":       imports(svc),
	}

	return template.RenderTpl(input, bindings)
}
