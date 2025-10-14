package v1

import (
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
	ID      string                   `json:"id"`
	Name    string                   `json:"name"`
	Type    console.AgentRuntimeType `json:"type"`
	AiProxy bool                     `json:"aiProxy"`
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
			ID:      fragment.Runtime.ID,
			Name:    fragment.Runtime.Name,
			Type:    fragment.Runtime.Type,
			AiProxy: fragment.Runtime.AiProxy != nil && *fragment.Runtime.AiProxy,
		}
	}

	return run
}

func (ar *AgentRun) IsProxyEnabled() bool {
	return ar.Runtime != nil && ar.Runtime.AiProxy
}
