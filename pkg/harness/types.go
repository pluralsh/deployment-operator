package harness

import (
	gqlclient "github.com/pluralsh/console-client-go"
)

type StackRun struct {
	ID          string
	Type        gqlclient.StackType
	Tarball     string
	Steps       []*gqlclient.RunStepFragment
	Files       []*gqlclient.StackFileFragment
	// TODO: Do we need to set it up somehow?
	Environment []*gqlclient.StackEnvironmentFragment
}

func (in *StackRun) FromStackRunBaseFragment(fragment *gqlclient.StackRunBaseFragment) *StackRun {
	return &StackRun{
		ID:      fragment.ID,
		Type:    fragment.Type,
		Tarball: fragment.Tarball,
		Steps:   fragment.Steps,
		//Files:         fragment.Files,
		// TODO: Files can't be stored currently. Mocking.
		Files: []*gqlclient.StackFileFragment{
			{
				Path:    "test.yml",
				Content: "value: 123",
			},
		},
		Environment: fragment.Environment,
	}
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
