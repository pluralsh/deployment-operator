package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/pluralsh/controller-reconcile-helper/pkg/patch"
	v1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PipelineGateScope struct {
	Client       client.Client
	PipelineGate *v1alpha1.PipelineGate

	ctx         context.Context
	patchHelper *patch.Helper
}

func (p *PipelineGateScope) PatchObject() error {
	return p.patchHelper.Patch(p.ctx, p.PipelineGate)
}

func NewPipelineGateScope(ctx context.Context, client client.Client, gate *v1alpha1.PipelineGate) (*PipelineGateScope, error) {
	if gate == nil {
		return nil, errors.New("failed to create new pipeline scope, got nil pipeline")
	}

	helper, err := patch.NewHelper(gate, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create new pipeline scope, go error: %s", err)
	}

	return &PipelineGateScope{
		Client:       client,
		PipelineGate: gate,
		ctx:          ctx,
		patchHelper:  helper,
	}, nil
}
