package opencode

import (
	"github.com/pluralsh/polly/containers"
	"github.com/sst/opencode-sdk-go"

	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
)

const (
	defaultOpenCodePort  = "4096"
	defaultAnalysisAgent = "analysis"
	defaultWriteAgent    = "autonomous"
	defaultProvider      = "plural"
	defaultModel         = string(ModelGPT41Mini)
)

type Provider string

const (
	ProviderPlural Provider = "plural"
	ProviderOpenAI Provider = "openai" // TODO: extend controller to inject required config via env vars
)

type Model string

const (
	ModelGPT41Mini Model = "gpt-4.1-mini"
	ModelGPT41     Model = "gpt-4.1"
	ModelGPT5Mini  Model = "gpt-5-mini"
	ModelGPT5      Model = "gpt-5"
)

var (
	supportedProviders = containers.ToSet([]Provider{ProviderPlural})
	supportedModels    = containers.ToSet([]Model{ModelGPT41Mini, ModelGPT41, ModelGPT5Mini, ModelGPT5})
)

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
