package template

import (
	console "github.com/pluralsh/console-client-go"
)

func configMap(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) map[string]string {
	res := map[string]string{}
	for _, config := range svc.Configuration {
		res[config.Name] = config.Value
	}

	return res
}

func contexts(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) map[string]map[string]interface{} {
	res := map[string]map[string]interface{}{}
	for _, context := range svc.Contexts {
		res[context.Name] = context.Configuration
	}
	return res
}
