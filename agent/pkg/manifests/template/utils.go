package template

import (
	console "github.com/pluralsh/console-client-go"
	"k8s.io/klog/v2/klogr"
)

var (
	log = klogr.New()
)

func configMap(svc *console.ServiceDeploymentExtended) map[string]string {
	res := map[string]string{}
	for _, config := range svc.Configuration {
		res[config.Name] = config.Value
	}

	return res
}
