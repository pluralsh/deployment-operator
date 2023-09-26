package template

import (
	console "github.com/pluralsh/console-client-go"
)

func configMap(svc *console.ServiceDeploymentExtended) map[string]string {
	res := map[string]string{}
	for _, config := range svc.Configuration {
		res[config.Name] = config.Value
	}

	return res
}
