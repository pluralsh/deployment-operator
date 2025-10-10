package opencode

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/imdario/mergo"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

type Server struct {
	sync.Mutex

	// session is a current opencode server session.
	session *opencode.Session

	// client is the opencode client used to communicate with the server.
	client *opencode.Client

	// executable is the opencode executable used to start the server.
	executable exec.Executable

	// port is a port the opencode server will listen on.
	port string

	// configFilePath is a path to the opencode config JSON file.
	configFilePath string

	// repositoryDir is a directory where the cloned repository is located.
	repositoryDir string

	// systemPrompt is a system prompt that will be used by the server.
	systemPrompt string

	// agent is an agent that will be used by the server.
	agent string

	// promptTimeout is a timeout for prompt requests.
	promptTimeout time.Duration

	// started is a flag indicating whether the server is started.
	started bool

	// cancel is a function that cancels the internal context.
	cancel context.CancelFunc
}

func (in *Server) Start(ctx context.Context, options ...exec.Option) error {
	in.Lock()
	defer in.Unlock()

	if in.started {
		return fmt.Errorf("server is already started")
	}

	configFilePath, err := filepath.Abs(in.configFilePath)
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to get absolute path to opencode config file")
		return err
	}

	internalCtx, cancel := context.WithCancel(ctx)
	in.cancel = cancel

	in.executable = exec.NewExecutable(
		"opencode",
		append(
			options,
			exec.WithEnv([]string{fmt.Sprintf("OPENCODE_CONFIG=%s", configFilePath)}),
			exec.WithArgs([]string{"serve", "--port", in.port}),
			exec.WithDir(in.repositoryDir),
		)...,
	)

	waitFn, err := in.executable.Start(internalCtx)
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "could not start opencode server")
		return err
	}

	if err = in.initSession(ctx); err != nil {
		return err
	}

	in.started = true
	klog.V(log.LogLevelDefault).InfoS("started opencode server", "port", in.port, "config", in.configFilePath, "repository", in.repositoryDir)

	go func() {
		err := waitFn()
		klog.V(log.LogLevelDefault).ErrorS(err, "opencode server stopped")
	}()

	return nil
}

func (in *Server) Listen(ctx context.Context) (<-chan Event, <-chan error) {
	msgChan := make(chan Event)
	errChan := make(chan error, 1)
	events := make(map[string]Event)

	klog.V(log.LogLevelDefault).InfoS("starting opencode server event listener")
	stream := in.client.Event.ListStreaming(ctx, opencode.EventListParams{})

	go func() {
		for stream.Next() {
			data := stream.Current()
			e, done := in.parseListenerData(data)
			if e == nil {
				continue
			}

			event, exists := events[e.ID]
			if !exists {
				if done {
					msgChan <- *e
				} else {
					events[e.ID] = *e
				}

				continue
			}

			_ = mergo.Merge(&event, e, mergo.WithOverride)
			events[e.ID] = event

			if done {
				msgChan <- event
				delete(events, e.ID)
			}
		}

		if err := stream.Err(); err != nil {
			errChan <- err
		}

		close(msgChan)
		close(errChan)
		klog.V(log.LogLevelDefault).InfoS("opencode event listener stopped")
	}()

	return msgChan, errChan
}

func (in *Server) parseListenerData(e opencode.EventListResponse) (*Event, bool) {
	switch e.Type {
	case opencode.EventListResponseTypeMessageUpdated:
		properties := e.Properties.(opencode.EventListResponseEventMessageUpdatedProperties)
		return &Event{
			ID:        properties.Info.ID,
			EventType: opencode.EventListResponseTypeMessageUpdated,
			Role:      lo.ToPtr(string(properties.Info.Role)),
			Mode:      lo.ToPtr(properties.Info.Mode),
			Model:     lo.ToPtr(properties.Info.ModelID),
			Provider:  lo.ToPtr(properties.Info.ProviderID),
		}, false
	case opencode.EventListResponseTypeMessagePartUpdated:
		properties := e.Properties.(opencode.EventListResponseEventMessagePartUpdatedProperties)
		files, _ := properties.Part.Files.([]string)
		state, _ := properties.Part.State.(opencode.ToolPartState)
		return &Event{
			ID:          properties.Part.MessageID,
			EventType:   opencode.EventListResponseTypeMessagePartUpdated,
			MessageType: lo.ToPtr(lo.Ternary(properties.Part.Type == "step-finish", "", properties.Part.Type)),
			Tool:        lo.ToPtr(properties.Part.Tool),
			Files:       files,
			State:       lo.ToPtr(state),
		}, properties.Part.Type == "step-finish"
	case opencode.EventListResponseTypeFileEdited:
		properties := e.Properties.(opencode.EventListResponseEventFileEditedProperties)
		return &Event{
			ID:        fmt.Sprintf("%s-%s", properties.File, lo.RandomString(5, lo.AlphanumericCharset)),
			EventType: opencode.EventListResponseTypeFileEdited,
			Files:     []string{properties.File},
		}, true
	default:
		return nil, false
	}
}

func (in *Server) Prompt(ctx context.Context, prompt string) (<-chan struct{}, <-chan error) {
	done := make(chan struct{})
	errChan := make(chan error)

	// TODO: remove after testing
	prompt = "Create or update main README.md file based on the contents of repository and then create a pull request with the proposed changes for further review."

	go func() {
		klog.V(log.LogLevelExtended).InfoS("sending prompt", "prompt", prompt)
		res, err := in.client.Session.Prompt(ctx, in.session.ID, opencode.SessionPromptParams{
			Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
				opencode.TextPartInputParam{
					Text: opencode.F(prompt),
					Type: opencode.F(opencode.TextPartInputTypeText),
				},
			}),
			System: opencode.F(in.systemPrompt),
			Agent:  opencode.F(in.agent),
			Model: opencode.F(opencode.SessionPromptParamsModel{
				ModelID:    opencode.F(defaultModelID),
				ProviderID: opencode.F(defaultProviderID),
			}),
		}, option.WithRequestTimeout(in.promptTimeout))
		if err != nil {
			errChan <- err
			close(errChan)
			close(done)
			return
		}

		close(done)
		close(errChan)
		klog.V(log.LogLevelExtended).InfoS("prompt sent successfully", "response", res)
	}()

	return done, errChan
}

func (in *Server) Stop() {
	in.Lock()
	defer in.Unlock()

	if !in.started {
		return
	}

	in.cancel()
	in.started = false
	klog.V(log.LogLevelDefault).InfoS("stopped opencode server")
}

func (in *Server) Restart(ctx context.Context, options ...exec.Option) error {
	in.Stop()
	return in.Start(ctx, options...)
}

func (in *Server) initSession(ctx context.Context) (err error) {
	if in.session != nil {
		if _, err = in.client.Session.Delete(ctx, in.session.ID, opencode.SessionDeleteParams{}); err != nil {
			return err
		}
	}

	in.session, err = in.client.Session.New(ctx, opencode.SessionNewParams{
		Title: opencode.F("Plural Agent Run"),
	})
	if err != nil {
		return err
	}

	return nil
}

func NewServer(port, configFilePath, repositoryDir string, mode console.AgentRunMode) *Server {
	return &Server{
		port:           port,
		configFilePath: configFilePath,
		repositoryDir:  repositoryDir,
		client:         opencode.NewClient(option.WithBaseURL(fmt.Sprintf("http://localhost:%s", port))),
		// TODO: use correct prompt/agent based on mode
		systemPrompt:  lo.Ternary(mode == console.AgentRunModeAnalyze, systemPromptWriter, systemPromptWriter),
		agent:         lo.Ternary(mode == console.AgentRunModeAnalyze, defaultWriteAgent, defaultWriteAgent),
		promptTimeout: 10 * time.Minute,
	}
}
