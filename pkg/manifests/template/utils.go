package template

import (
	"strings"

	console "github.com/pluralsh/console/go/client"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
)

func isTemplated(svc *console.ServiceDeploymentForAgent) bool {
	if svc.Templated != nil {
		return *svc.Templated
	}
	// default true
	return true
}

func clusterConfiguration(cluster *console.ServiceDeploymentForAgent_Cluster) map[string]interface{} {
	res := map[string]interface{}{
		"ID":             cluster.ID,
		"Self":           cluster.Self,
		"Handle":         cluster.Handle,
		"Name":           cluster.Name,
		"Version":        cluster.Version,
		"CurrentVersion": cluster.CurrentVersion,
		"KasUrl":         cluster.KasURL,
		"Tags":           tagsMap(cluster.Tags),
		"Metadata":       cluster.Metadata,
		"Distro":         cluster.Distro,
		"ConsoleDNS":     args.ConsoleDNS(),
	}

	for k, v := range res {
		res[strings.ToLower(k)] = v
	}
	res["kasUrl"] = cluster.KasURL
	res["currentVersion"] = cluster.CurrentVersion

	return res
}

func tagsMap(tags []*console.ClusterTags) map[string]string {
	res := map[string]string{}
	for _, tag := range tags {
		res[tag.Name] = tag.Value
	}
	return res
}

func configMap(svc *console.ServiceDeploymentForAgent) map[string]string {
	res := map[string]string{}
	for _, config := range svc.Configuration {
		res[config.Name] = config.Value
	}

	return res
}

func contexts(svc *console.ServiceDeploymentForAgent) map[string]map[string]interface{} {
	res := map[string]map[string]interface{}{}
	for _, context := range svc.Contexts {
		res[context.Name] = context.Configuration
	}
	return res
}

func imports(svc *console.ServiceDeploymentForAgent) map[string]map[string]string {
	res := map[string]map[string]string{}
	for _, imp := range svc.Imports {
		res[imp.Stack.Name] = map[string]string{}
		for _, out := range imp.Outputs {
			res[imp.Stack.Name][out.Name] = out.Value
		}
	}
	return res
}
