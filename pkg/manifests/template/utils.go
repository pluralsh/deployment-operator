package template

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
)

func isTemplated(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) bool {
	if svc.Templated != nil {
		return *svc.Templated
	}
	// default true
	return true
}

func clusterConfiguration(cluster *console.GetServiceDeploymentForAgent_ServiceDeployment_Cluster) map[string]interface{} {
	res := map[string]interface{}{
		"ID":             cluster.ID,
		"Self":           cluster.Self,
		"Handle":         cluster.Handle,
		"Name":           cluster.Name,
		"Version":        cluster.Version,
		"CurrentVersion": cluster.CurrentVersion,
		"KasUrl":         cluster.KasURL,
		"Metadata":       cluster.Metadata,
	}

	for k, v := range res {
		res[strings.ToLower(k)] = v
	}
	res["kasUrl"] = cluster.KasURL
	res["currentVersion"] = cluster.CurrentVersion

	return res
}

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

func imports(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) map[string]map[string]string {
	res := map[string]map[string]string{}
	for _, imp := range svc.Imports {
		res[imp.Stack.Name] = map[string]string{}
		for _, out := range imp.Outputs {
			res[imp.Stack.Name][out.Name] = out.Value
		}
	}
	return res
}
