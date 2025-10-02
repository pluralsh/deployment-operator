package common

import (
	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

type Component struct {
	UID                  string
	ParentUID            string
	Group                string
	Version              string
	Kind                 string
	Name                 string
	Namespace            string
	Status               string
	ServiceID            string
	SyncPhase            string
	ManifestSHA          string
	TransientManifestSHA string
	ApplySHA             string
	ServerSHA            string
}

// ShouldApply determines if a resource should be applied.
// Resource should be applied if at least one of the following conditions is met:
// - any of the SHAs (Server, Apply, or Manifest) are not set
// - the current server SHA differs from stored apply SHA (indicating resource changed in cluster)
// - the new manifest SHA differs from stored manifest SHA (indicating the manifest has changed)
// - the resource is not in a running state
func (in *Component) ShouldApply(newManifestSHA string) bool {
	return in.ServerSHA == "" || in.ApplySHA == "" || in.ManifestSHA == "" ||
		in.ServerSHA != in.ApplySHA || newManifestSHA != in.ManifestSHA ||
		client.ComponentState(in.Status) != client.ComponentStateRunning
}

func (in *Component) ToComponentAttributes() client.ComponentAttributes {
	return client.ComponentAttributes{
		UID:       lo.ToPtr(in.UID),
		Synced:    true,
		Group:     in.Group,
		Version:   in.Version,
		Kind:      in.Kind,
		Name:      in.Name,
		Namespace: in.Namespace,
		State:     lo.ToPtr(client.ComponentState(in.Status)),
	}
}
