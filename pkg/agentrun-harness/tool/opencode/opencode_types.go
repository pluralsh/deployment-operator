package opencode

import (
	"encoding/json"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"

	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	toolv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

const (
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

// Opencode implements toolv1.Tool interface.
type Opencode struct {
	toolv1.DefaultTool

	// model is the AI model used by opencode.
	model Model

	// provider is the AI provider used by opencode.
	provider Provider

	// executable is the opencode executable used to call CLI.
	executable exec.Executable

	// timeout bounds a single opencode run invocation.
	timeout time.Duration

	// onMessage is a callback called when a new message is received.
	onMessage func(message *console.AgentMessageAttributes)
}

type StreamEventType string

const (
	StreamEventTypeStepStart  StreamEventType = "step_start"
	StreamEventTypeToolUse    StreamEventType = "tool_use"
	StreamEventTypeStepFinish StreamEventType = "step_finish"
	StreamEventTypeText       StreamEventType = "text"
	StreamEventTypeError      StreamEventType = "error"
)

type StreamPartType string

const (
	StreamPartTypeText       StreamPartType = "text"
	StreamPartTypeTool       StreamPartType = "tool"
	StreamPartTypeStepStart  StreamPartType = "step-start"
	StreamPartTypeStepFinish StreamPartType = "step-finish"
)

type StreamToolStatus string

const (
	StreamToolStatusRunning   StreamToolStatus = "running"
	StreamToolStatusCompleted StreamToolStatus = "completed"
	StreamToolStatusPending   StreamToolStatus = "pending"
	StreamToolStatusError     StreamToolStatus = "error"
)

type EventListResponse struct {
	Type      StreamEventType `json:"type"`
	Timestamp int64           `json:"timestamp"`
	SessionID string          `json:"sessionID"`
	Part      *StreamPart     `json:"part,omitempty"`
	Error     *StreamError    `json:"error,omitempty"`
}

type StreamPart struct {
	ID        string           `json:"id"`
	SessionID string           `json:"sessionID"`
	MessageID string           `json:"messageID"`
	CallID    string           `json:"callID,omitempty"`
	Type      StreamPartType   `json:"type"`
	Text      string           `json:"text,omitempty"`
	Tool      string           `json:"tool,omitempty"`
	Cost      float64          `json:"cost,omitempty"`
	Tokens    *StreamTokens    `json:"tokens,omitempty"`
	State     *StreamToolState `json:"state,omitempty"`
}

type StreamTokens struct {
	Total     float64 `json:"total"`
	Input     float64 `json:"input"`
	Output    float64 `json:"output"`
	Reasoning float64 `json:"reasoning"`
}

type StreamToolState struct {
	Status StreamToolStatus `json:"status"`
	Input  json.RawMessage  `json:"input,omitempty"`
	Output string           `json:"output,omitempty"`
}

type StreamErrorData struct {
	Message string `json:"message"`
}

type StreamError struct {
	Name string           `json:"name"`
	Data *StreamErrorData `json:"data,omitempty"`
}

type Event struct {
	ID      string
	Message *console.AgentMessageAttributes
	Done    bool

	seenToolCalls map[string]struct{}
}

type streamState struct {
	events map[string]*Event
}

func (in *Event) FromEventResponse(e EventListResponse) {
	if in.Message == nil {
		in.Message = &console.AgentMessageAttributes{}
	}

	if e.Part == nil {
		return
	}

	in.ID = e.Part.MessageID
	in.Message.Role = console.AiRoleAssistant

	switch e.Part.Type {
	case StreamPartTypeText:
		if len(e.Part.Text) > len(in.Message.Message) {
			in.Message.Message = e.Part.Text
		}
	case StreamPartTypeTool:
		in.fromToolState(e.Part)
		if len(in.Message.Message) == 0 {
			in.Message.Message = "Called tool"
		}
	case StreamPartTypeStepFinish:
		in.fromTokens(e.Part.Tokens, e.Part.Cost)
		in.Done = true
	}
}

func (in *Event) fromToolState(part *StreamPart) {
	if part == nil || part.State == nil || len(part.Tool) == 0 {
		return
	}

	if in.Message.Metadata == nil {
		in.Message.Metadata = &console.AgentMessageMetadataAttributes{}
	}

	if in.Message.Metadata.Tool == nil {
		in.Message.Metadata.Tool = &console.AgentMessageToolAttributes{}
	}

	in.Message.Metadata.Tool.Name = lo.ToPtr(part.Tool)
	in.Message.Metadata.Tool.State = lo.ToPtr(toAgentToolState(part.State.Status))
	in.Message.Metadata.Tool.Output = lo.ToPtr(part.State.Output)

	if len(part.State.Input) > 0 && string(part.State.Input) != "null" {
		in.Message.Metadata.Tool.Input = lo.ToPtr(string(part.State.Input))
	}

	if in.seenToolCalls == nil {
		in.seenToolCalls = make(map[string]struct{})
	}

	callID := part.CallID
	if callID == "" {
		callID = part.ID
	}

	if _, exists := in.seenToolCalls[callID]; exists {
		return
	}

	if len(in.Message.Message) > 0 {
		in.Message.Message += "\n"
	}
	in.Message.Message += "Called tool " + part.Tool
	in.seenToolCalls[callID] = struct{}{}
}

func (in *Event) fromTokens(tokens *StreamTokens, cost float64) {
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

func (in *Event) Sanitize() {
	if len(in.Message.Message) == 0 {
		in.Message.Message = "__plrl_ignore__"
	}
}

func toAgentToolState(state StreamToolStatus) console.AgentMessageToolState {
	switch state {
	case StreamToolStatusRunning:
		return console.AgentMessageToolStateRunning
	case StreamToolStatusCompleted:
		return console.AgentMessageToolStateCompleted
	case StreamToolStatusPending:
		return console.AgentMessageToolStatePending
	case StreamToolStatusError:
		return console.AgentMessageToolStateError
	default:
		return console.AgentMessageToolStateCompleted
	}
}
