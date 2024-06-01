package v1

import (
	"fmt"

	gqlclient "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
)

type StackRun struct {
	ID          string
	Status      gqlclient.StackStatus
	Type        gqlclient.StackType
	Tarball     string
	Steps       []*gqlclient.RunStepFragment
	Files       []*gqlclient.StackFileFragment
	Environment []*gqlclient.StackEnvironmentFragment
	ExecWorkDir *string
	Approval    bool
	ApprovedAt  *string
	ManageState bool
	Creds       *gqlclient.StackRunBaseFragment_PluralCreds
	StateUrls   *gqlclient.StackRunBaseFragment_StateUrls
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
		Approval:    fragment.Approval != nil && *fragment.Approval,
		ApprovedAt:  fragment.ApprovedAt,
		ExecWorkDir: fragment.Workdir,
		ManageState: fragment.ManageState != nil && *fragment.ManageState,
		Creds:       fragment.PluralCreds,
		StateUrls:   fragment.StateUrls,
	}
}

// Env parses the StackRun.Environment as a list of strings.
// Each entry is of the form "key=value". Automatically adds Plural env vars if creds were configured.
func (in *StackRun) Env() []string {
	env := make([]string, len(in.Environment))

	for i, e := range in.Environment {
		env[i] = fmt.Sprintf("%s=%s", e.Name, e.Value)
	}

	if in.Creds != nil {
		env = append(env, fmt.Sprintf("PLURAL_ACCESS_TOKEN=%s", lo.FromPtr(in.Creds.Token)))
		env = append(env, fmt.Sprintf("PLURAL_CONSOLE_URL=%s", lo.FromPtr(in.Creds.URL)))
	}

	return env
}

type Lifecycle string

const (
	LifecyclePreStart  Lifecycle = "pre-start"
	LifecyclePostStart Lifecycle = "post-start"
)

type HookFunction func() error
