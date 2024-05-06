package harness

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

type StackRunOutput struct {
	ID     string
	Status gqlclient.StackStatus
	State  *gqlclient.StackStateAttributes
	Output []*gqlclient.StackOutputAttributes
	Errors []*gqlclient.ServiceErrorAttributes
}

func (in *StackRunOutput) ToStackRunAttributes() *gqlclient.StackRunAttributes {
	return &gqlclient.StackRunAttributes{
		Status: in.Status,
		State:  in.State,
		Output: in.Output,
		Errors: in.Errors,
	}
}
