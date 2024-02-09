package template

import (
	"github.com/pluralsh/deployment-operator/pkg/client"
)

func configMap(svc *client.ServiceDeployment) map[string]string {
	res := map[string]string{}
	for _, config := range svc.Configuration {
		res[config.Name] = config.Value
	}

	return res
}
