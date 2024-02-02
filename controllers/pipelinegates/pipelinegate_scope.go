package pipelinegates

//import (
//	"context"
//	"errors"
//	"fmt"
//
//	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/api/pipelines/v1alpha1"
//	"sigs.k8s.io/cluster-api/util/patch"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//)
//
//type PipelineScope struct {
//	Client   client.Client
//	Pipeline *pipelinesv1alpha1.PipelineGate
//
//	ctx         context.Context
//	patchHelper *patch.Helper
//}
//
//func (p *PipelineScope) PatchObject() error {
//	return p.patchHelper.Patch(p.ctx, p.Pipeline)
//}
//
//func NewPipelineGateScope(ctx context.Context, client client.Client, pipeline *pipelinesv1alpha1.PipelineGate) (*PipelineScope, error) {
//	if pipeline == nil {
//		return nil, errors.New("failed to create new pipeline scope, got nil pipeline")
//	}
//
//	helper, err := patch.NewHelper(pipeline, client)
//	if err != nil {
//		return nil, fmt.Errorf("failed to create new pipeline scope, go error: %s", err)
//	}
//
//	return &PipelineScope{
//		Client:      client,
//		Pipeline:    pipeline,
//		ctx:         ctx,
//		patchHelper: helper,
//	}, nil
//}
//
