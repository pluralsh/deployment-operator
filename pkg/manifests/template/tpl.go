package template

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/template"
)

func renderTpl(input []byte, svc *console.GetServiceDeploymentForAgent_ServiceDeployment) ([]byte, error) {
	bindings := map[string]interface{}{
		"Configuration": configMap(svc),
		"Cluster":       clusterConfiguration(svc.Cluster),
		"Contexts":      contexts(svc),
	}

	return template.RenderTpl(input, bindings)
}
