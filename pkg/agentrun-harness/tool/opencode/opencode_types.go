package opencode

import (
	"github.com/sst/opencode-sdk-go"

	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
)

const (
	defaultOpenCodePort  = "4096"
	defaultAnalysisAgent = "analysis"
	defaultWriteAgent    = "autonomous"
)

type Provider string

func (in Provider) Endpoint() string {
	switch in {
	case ProviderOpenAI:
		return "https://api.openai.com/v1"
	}

	return ""
}

const (
	ProviderPlural Provider = "plural"
	ProviderOpenAI Provider = "openai"
)

func DefaultProvider() Provider {
	switch helpers.GetEnv(controller.EnvOpenCodeProvider, string(ProviderPlural)) {
	case string(ProviderPlural):
		return ProviderPlural
	case string(ProviderOpenAI):
		return ProviderOpenAI
	default:
		return ProviderPlural
	}
}

type Model string

const (
	ModelGPT41Mini Model = "gpt-4.1-mini"
	ModelGPT41     Model = "gpt-4.1"
	ModelGPT5Mini  Model = "gpt-5-mini"
	ModelGPT5      Model = "gpt-5"
)

func DefaultModel() Model {
	switch helpers.GetEnv(controller.EnvOpenCodeModel, string(ModelGPT41Mini)) {
	case string(ModelGPT41Mini):
		return ModelGPT41Mini
	case string(ModelGPT41):
		return ModelGPT41
	case string(ModelGPT5Mini):
		return ModelGPT5Mini
	case string(ModelGPT5):
		return ModelGPT5
	default:
		return ModelGPT41Mini
	}
}

// Opencode implements v1.Tool interface.
type Opencode struct {
	// dir is a working directory used to run opencode.
	dir string

	// repositoryDir is a directory where the cloned repository is located.
	repositoryDir string

	// port is a port the opencode server will listen on.
	port string

	// run is the agent run that is being processed.
	run *v1.AgentRun

	// server is the opencode server.
	server *Server

	// client is the opencode client.
	client *opencode.Client

	// errorChan is a channel that returns an error if the tool failed
	errorChan chan error

	// finishedChan is a channel that gets closed when the tool is finished.
	finishedChan chan struct{}

	// startedChan is a channel that gets closed when the opencode server is started.
	startedChan chan struct{}
}

type Event struct {
	ID          string
	EventType   opencode.EventListResponseType
	MessageType *opencode.PartType
	Role        *string
	Mode        *string
	Model       *string
	Provider    *string
	Tool        *string
	Files       []string
	State       *opencode.ToolPartState
}
