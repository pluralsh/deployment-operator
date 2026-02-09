package v1

import (
	console "github.com/pluralsh/console/go/client"

	"github.com/pluralsh/deployment-operator/internal/helpers"
)

const (
	EnvDindEnabled    = "DIND_ENABLED"
	EnvBrowserEnabled = "BROWSER_ENABLED"
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

	DindEnabled    bool
	BrowserEnabled bool
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
		Runtime:     &AgentRuntime{},
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

	if helpers.GetPluralEnvBool(EnvDindEnabled, false) {
		run.DindEnabled = true
	}

	if helpers.GetPluralEnvBool(EnvBrowserEnabled, false) {
		run.BrowserEnabled = true
	}

	return run
}

func (ar *AgentRun) IsProxyEnabled() bool {
	return ar.Runtime != nil && ar.Runtime.AiProxy
}
