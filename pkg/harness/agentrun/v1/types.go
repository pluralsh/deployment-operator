package v1

import (
	"fmt"

	console "github.com/pluralsh/console/go/client"
)

type AgentRun struct {
	ID         string                 `json:"id"`
	Prompt     string                 `json:"prompt"`
	Repository string                 `json:"repository"`
	Mode       console.AgentRunMode   `json:"mode"`
	Status     console.AgentRunStatus `json:"status"`
	FlowID     *string                `json:"flowId,omitempty"`

	// Credentials for SCM and Plural Console
	ScmCreds    *console.ScmCredentialFragment `json:"scmCreds,omitempty"`
	PluralCreds *console.PluralCredsFragment   `json:"pluralCreds,omitempty"`

	// Runtime information
	Runtime *AgentRuntime `json:"runtime,omitempty"`
}

type AgentRuntime struct {
	ID   string                   `json:"id"`
	Name string                   `json:"name"`
	Type console.AgentRuntimeType `json:"type"`
}

// FromAgentRunFragment converts Console API fragment to harness type
func (ar *AgentRun) FromAgentRunFragment(fragment *console.AgentRunFragment) *AgentRun {
	run := &AgentRun{
		ID:          fragment.ID,
		Prompt:      fragment.Prompt,
		Repository:  fragment.Repository,
		Mode:        fragment.Mode,
		Status:      fragment.Status,
		ScmCreds:    fragment.ScmCreds,
		PluralCreds: fragment.PluralCreds,
	}

	if fragment.Flow != nil {
		run.FlowID = &fragment.Flow.ID
	}

	if fragment.Runtime != nil {
		run.Runtime = &AgentRuntime{
			ID:   fragment.Runtime.ID,
			Name: fragment.Runtime.Name,
			Type: fragment.Runtime.Type,
		}
	}

	return run
}

// Env returns environment variables for the agent run
func (ar *AgentRun) Env() []string {
	env := []string{
		fmt.Sprintf("AGENT_RUN_ID=%s", ar.ID),
		fmt.Sprintf("AGENT_PROMPT=%s", ar.Prompt),
		fmt.Sprintf("AGENT_REPOSITORY=%s", ar.Repository),
		fmt.Sprintf("AGENT_MODE=%s", ar.Mode),
	}

	// Add Plural credentials if available
	if ar.PluralCreds != nil {
		if ar.PluralCreds.Token != nil {
			env = append(env, fmt.Sprintf("PLURAL_ACCESS_TOKEN=%s", *ar.PluralCreds.Token))
		}
		if ar.PluralCreds.URL != nil {
			env = append(env, fmt.Sprintf("PLURAL_CONSOLE_URL=%s", *ar.PluralCreds.URL))
		}
	}

	// Add SCM credentials if available
	if ar.ScmCreds != nil {
		env = append(env, fmt.Sprintf("SCM_USERNAME=%s", ar.ScmCreds.Username))
		env = append(env, fmt.Sprintf("SCM_TOKEN=%s", ar.ScmCreds.Token))
	}

	return env
}

// IsAnalyzeMode returns true if this is an analysis-only run
func (ar *AgentRun) IsAnalyzeMode() bool {
	return ar.Mode == console.AgentRunModeAnalyze
}

// IsWriteMode returns true if this is a code-writing run
func (ar *AgentRun) IsWriteMode() bool {
	return ar.Mode == console.AgentRunModeWrite
}

type Lifecycle string

const (
	LifecyclePreStart  Lifecycle = "pre-start"
	LifecyclePostStart Lifecycle = "post-start"
)

type HookFunction func() error
