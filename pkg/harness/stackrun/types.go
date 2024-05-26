package stackrun

import (
	"fmt"

	gqlclient "github.com/pluralsh/console-client-go"
)

type StackRun struct {
	ID          string
	Status      gqlclient.StackStatus
	Type        gqlclient.StackType
	Tarball     string
	Steps       []*gqlclient.RunStepFragment
	Files       []*gqlclient.StackFileFragment
	Environment []*gqlclient.StackEnvironmentFragment
	Approval    bool
	ApprovedAt  *string
	Workdir     *string
}

func (in *StackRun) FromStackRunBaseFragment(fragment *gqlclient.StackRunBaseFragment) *StackRun {
	return &StackRun{
		ID:          fragment.ID,
		Status:      fragment.Status,
		Type:        fragment.Type,
		Tarball:     fragment.Tarball,
		Steps:       fragment.Steps,
		Files:       fragment.Files,
		Environment: fragment.Environment,
		Workdir:     fragment.Workdir,
		Approval:    fragment.Approval != nil && *fragment.Approval,
		ApprovedAt:  fragment.ApprovedAt,
	}
}

// Env parses the StackRun.Environment as a list of strings.
// Each entry is of the form "key=value".
func (in *StackRun) Env() []string {
	result := make([]string, len(in.Environment))

	for i, e := range in.Environment {
		result[i] = fmt.Sprintf("%s=%s", e.Name, e.Value)
	}

	return result
}

type Lifecycle string

const (
	LifecyclePreStart  Lifecycle = "pre-start"
	LifecyclePostStart Lifecycle = "post-start"
)

type HookFunction func() error