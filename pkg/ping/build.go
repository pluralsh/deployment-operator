package ping

import (
	"strings"

	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/version"
)

func pingAttributes(info *version.Info, pods []string, minKubeletVersion *string) console.ClusterPing {
	vs := strings.Split(info.GitVersion, "-")
	return console.ClusterPing{
		CurrentVersion: strings.TrimPrefix(vs[0], "v"),
		Distro:         lo.ToPtr(findDistro(append(pods, info.GitVersion))),
		KubeletVersion: minKubeletVersion,
	}
}
