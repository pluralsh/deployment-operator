package template

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/template"
)

func renderTpl(filePath string, svc *console.GetServiceDeploymentForAgent_ServiceDeployment) ([]byte, error) {

	bindings := map[string]interface{}{
		"Configuration": configMap(svc),
		"Cluster":       clusterConfiguration(svc.Cluster),
		"Contexts":      contexts(svc),
	}

	return template.RenderTpl(filePath, bindings)
}
