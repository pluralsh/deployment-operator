package ping

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
)

func findDistro(vals []string) console.ClusterDistro {
	for _, v := range vals {
		if dist, ok := distro(v); ok {
			return dist
		}
	}

	return console.ClusterDistroGeneric
}

func distro(val string) (console.ClusterDistro, bool) {
	if strings.Contains(val, "eks") {
		return console.ClusterDistroEks, true
	}

	if strings.Contains(val, "aks") || strings.Contains(val, "azure") {
		return console.ClusterDistroAks, true
	}

	if strings.Contains(val, "gke") {
		return console.ClusterDistroGke, true
	}

	if strings.Contains(val, "k3s") {
		return console.ClusterDistroK3s, true
	}

	if strings.Contains(val, "rke") {
		return console.ClusterDistroRke, true
	}

	return console.ClusterDistroGeneric, false
}
