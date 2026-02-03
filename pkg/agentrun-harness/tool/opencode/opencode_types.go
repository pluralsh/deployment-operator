package opencode

import (
	"encoding/json"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
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
	if in == ProviderOpenAI {
		return "https://api.openai.com/v1"
	}

	return ""
}

const (
	ProviderPlural  Provider = "plural"
	ProviderOpenAI  Provider = "openai"
	defaultProvider          = ProviderPlural
)

func DefaultProvider(proxyEnabled bool) Provider {
	if proxyEnabled {
		return ProviderPlural
	}

	switch helpers.GetEnv(controller.EnvOpenCodeProvider, string(defaultProvider)) {
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
	ModelGPT41   Model = "gpt-4.1"
	ModelGPT5    Model = "gpt-5"
	ModelGPT51   Model = "gpt-5.1"
	ModelGPT52   Model = "gpt-5.2"
	defaultModel       = ModelGPT5
)

func DefaultModel() Model {
	switch helpers.GetEnv(controller.EnvOpenCodeModel, string(defaultModel)) {
	case string(ModelGPT41):
		return ModelGPT41
	case string(ModelGPT5):
		return ModelGPT5
	case string(ModelGPT51):
		return ModelGPT51
	case string(ModelGPT52):
		return ModelGPT52
	default:
		return defaultModel
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

	// model is the AI model used by opencode.
	model Model

	// provider is the AI provider used by opencode.
	provider Provider

	// server is the opencode server.
	server *Server

	// client is the opencode client.
	client *opencode.Client

	// onMessage is a callback called when a new message is received.
	onMessage func(message *console.AgentMessageAttributes)

	// errorChan is a channel that returns an error if the tool failed
	errorChan chan error

	// finishedChan is a channel that gets closed when the tool is finished.
	finishedChan chan struct{}

	// startedChan is a channel that gets closed when the opencode server is started.
	startedChan chan struct{}
}

type Event struct {
	ID      string
	Message *console.AgentMessageAttributes
	Done    bool
}

func (in *Event) FromEventResponse(e opencode.EventListResponse) {
	if in.Message == nil {
		in.Message = &console.AgentMessageAttributes{}
	}

	switch e.Type {
	case opencode.EventListResponseTypeMessageUpdated:
		in.fromMessageUpdated(e.Properties.(opencode.EventListResponseEventMessageUpdatedProperties))
	case opencode.EventListResponseTypeMessagePartUpdated:
		in.fromMessagePartUpdated(e.Properties.(opencode.EventListResponseEventMessagePartUpdatedProperties))
	}
}

func (in *Event) Sanitize() {
	if len(in.Message.Message) == 0 {
		in.Message.Message = "__plrl_ignore__"
	}
}

func (in *Event) fromMessageUpdated(e opencode.EventListResponseEventMessageUpdatedProperties) {
	in.ID = e.Info.ID
	in.Message.Role = in.toRole(e.Info.Role)

	tokens, ok := e.Info.Tokens.(opencode.AssistantMessageTokens)
	in.fromAssistantMessageTokens(lo.Ternary(ok, &tokens, nil), e.Info.Cost)
}

func (in *Event) fromAssistantMessageTokens(tokens *opencode.AssistantMessageTokens, cost float64) {
	if in.Message.Cost == nil {
		in.Message.Cost = &console.AgentMessageCostAttributes{}
	}

	if in.Message.Cost.Total < cost {
		in.Message.Cost.Total = cost
	}

	if tokens == nil {
		return
	}

	if in.Message.Cost.Tokens == nil {
		in.Message.Cost.Tokens = &console.AgentMessageTokensAttributes{}
	}

	in.Message.Cost.Tokens.Input = lo.ToPtr(tokens.Input)
	in.Message.Cost.Tokens.Output = lo.ToPtr(tokens.Output)
	in.Message.Cost.Tokens.Reasoning = lo.ToPtr(tokens.Reasoning)
}

func (in *Event) toRole(role opencode.MessageRole) console.AiRole {
	switch role {
	case opencode.MessageRoleUser:
		return console.AiRoleUser
	case opencode.MessageRoleAssistant:
		return console.AiRoleAssistant
	}

	return console.AiRoleAssistant
}

func (in *Event) fromMessagePartUpdated(e opencode.EventListResponseEventMessagePartUpdatedProperties) {
	in.ID = e.Part.MessageID
	in.Message.Message = lo.Ternary(len(in.Message.Message) > len(e.Part.Text), in.Message.Message, e.Part.Text)

	if e.Part.Type == opencode.PartTypeStepFinish {
		in.Done = true
	}

	tokens, ok := e.Part.Tokens.(opencode.StepFinishPartTokens)
	in.fromStepFinishPartTokens(lo.Ternary(ok, &tokens, nil), e.Part.Cost)

	tool, ok := e.Part.State.(opencode.ToolPartState)
	in.fromToolPartState(lo.Ternary(ok, &tool, nil), e.Part.Tool)

	file, ok := e.Part.Source.(opencode.FilePartSource)
	in.fromFilePartSource(lo.Ternary(ok, &file, nil))

	reasoning, ok := e.Part.Source.(opencode.AgentPartSource)
	in.fromAgentPartSource(lo.Ternary(ok, &reasoning, nil))
}

func (in *Event) fromStepFinishPartTokens(tokens *opencode.StepFinishPartTokens, cost float64) {
	if in.Message.Cost == nil {
		in.Message.Cost = &console.AgentMessageCostAttributes{}
	}

	if in.Message.Cost.Total < cost {
		in.Message.Cost.Total = cost
	}

	if tokens == nil {
		return
	}

	if in.Message.Cost.Tokens == nil {
		in.Message.Cost.Tokens = &console.AgentMessageTokensAttributes{}
	}

	in.Message.Cost.Tokens.Input = lo.ToPtr(tokens.Input)
	in.Message.Cost.Tokens.Output = lo.ToPtr(tokens.Output)
	in.Message.Cost.Tokens.Reasoning = lo.ToPtr(tokens.Reasoning)
}

func (in *Event) fromToolPartState(tool *opencode.ToolPartState, name string) {
	if tool == nil || len(name) == 0 {
		return
	}

	if in.Message.Metadata == nil {
		in.Message.Metadata = &console.AgentMessageMetadataAttributes{}
	}

	if in.Message.Metadata.Tool == nil {
		in.Message.Metadata.Tool = &console.AgentMessageToolAttributes{}
	}

	in.Message.Metadata.Tool.Name = lo.ToPtr(name)
	in.Message.Metadata.Tool.State = lo.ToPtr(in.toAgentToolState(tool.Status))
	in.Message.Metadata.Tool.Output = lo.ToPtr(tool.Output)

	if tool.Input != nil {
		input, err := json.Marshal(tool.Input)
		if err != nil {
			return
		}

		in.Message.Metadata.Tool.Input = lo.ToPtr(string(input))
	}
}

func (in *Event) toAgentToolState(state opencode.ToolPartStateStatus) console.AgentMessageToolState {
	switch state {
	case opencode.ToolPartStateStatusRunning:
		return console.AgentMessageToolStateRunning
	case opencode.ToolPartStateStatusCompleted:
		return console.AgentMessageToolStateCompleted
	case opencode.ToolPartStateStatusPending:
		return console.AgentMessageToolStatePending
	case opencode.ToolPartStateStatusError:
		return console.AgentMessageToolStateError
	}

	return console.AgentMessageToolStateCompleted
}

func (in *Event) fromFilePartSource(file *opencode.FilePartSource) {
	if file == nil {
		return
	}

	if in.Message.Metadata == nil {
		in.Message.Metadata = &console.AgentMessageMetadataAttributes{}
	}

	if in.Message.Metadata.File == nil {
		in.Message.Metadata.File = &console.AgentMessageFileAttributes{}
	}

	in.Message.Metadata.File.Name = lo.ToPtr(file.Name)
	in.Message.Metadata.File.Text = lo.ToPtr(file.Text.Value)
	in.Message.Metadata.File.Start = lo.ToPtr(file.Text.Start)
	in.Message.Metadata.File.End = lo.ToPtr(file.Text.End)
}

func (in *Event) fromAgentPartSource(reasoning *opencode.AgentPartSource) {
	if reasoning == nil {
		return
	}

	if in.Message.Metadata == nil {
		in.Message.Metadata = &console.AgentMessageMetadataAttributes{}
	}

	if in.Message.Metadata.Reasoning == nil {
		in.Message.Metadata.Reasoning = &console.AgentMessageReasoningAttributes{}
	}

	in.Message.Metadata.Reasoning.Text = lo.ToPtr(reasoning.Value)
	in.Message.Metadata.Reasoning.Start = lo.ToPtr(reasoning.Start)
	in.Message.Metadata.Reasoning.End = lo.ToPtr(reasoning.End)
}
