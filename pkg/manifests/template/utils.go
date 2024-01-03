package template

import (
	"encoding/json"

	console "github.com/pluralsh/console-client-go"
)

func configMap(svc *console.ServiceDeploymentExtended) map[string]string {
	res := map[string]string{}
	for _, config := range svc.Configuration {
		res[config.Name] = config.Value
	}

	return res
}

func workers(svc *console.ServiceDeploymentExtended) any {
	var nodePools any
	for _, config := range svc.Configuration {
		if config.Name == "nodePools" {
			_ = json.Unmarshal([]byte(config.Value), &nodePools)
			break
		}
	}

	return nodePools
}
