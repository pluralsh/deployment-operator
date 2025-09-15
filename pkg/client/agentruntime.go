package client

import (
	"context"
	stderrors "errors"

	console "github.com/pluralsh/console/go/client"
	internalerror "github.com/pluralsh/deployment-operator/internal/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (c *client) GetAgentRuntime(ctx context.Context, id string) (*console.AgentRuntimeFragment, error) {
	// we assume that an empty id means the agent does not exist
	// this is to avoid making a call to the backend with an empty id
	if id == "" {
		return nil, errors.NewNotFound(schema.GroupResource{}, "")
	}
	response, err := c.consoleClient.GetAgentRuntime(ctx, id)
	if internalerror.IsNotFound(err) {
		return nil, errors.NewNotFound(schema.GroupResource{}, id)
	}
	if err == nil && (response == nil || response.AgentRuntime == nil) {
		return nil, errors.NewNotFound(schema.GroupResource{}, id)
	}
	if response == nil {
		return nil, err
	}

	return response.AgentRuntime, err
}

func (c *client) UpsertAgentRuntime(ctx context.Context, attrs console.AgentRuntimeAttributes) (*console.AgentRuntimeFragment, error) {
	response, err := c.consoleClient.UpsertAgentRuntime(ctx, attrs)
	if err != nil {
		return nil, err
	}
	return response.UpsertAgentRuntime, nil
}

func (c *client) DeleteAgentRuntime(ctx context.Context, id string) error {
	_, err := c.consoleClient.DeleteAgentRuntime(ctx, id)
	return err
}

func (c *client) ListAgentRuntime(ctx context.Context, after *string, first *int64, q *string, typeArg *console.AgentRuntimeType) (*console.AgentRuntimeConnectionFragment, error) {
	resp, err := c.consoleClient.ListAgentRuntimes(ctx, after, first, nil, nil, q, typeArg)
	if err != nil {
		return nil, err
	}
	if resp.AgentRuntimes == nil {
		return nil, stderrors.New("the response from ListAgentRuntimes is nil")
	}
	return resp.AgentRuntimes, nil
}
