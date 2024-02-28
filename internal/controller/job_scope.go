package controller

import (
	"context"
	"errors"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/pluralsh/controller-reconcile-helper/pkg/patch"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type JobScope struct {
	Client client.Client
	Job    *batchv1.Job

	ctx         context.Context
	patchHelper *patch.Helper
}

func (p *JobScope) PatchObject() error {
	return p.patchHelper.Patch(p.ctx, p.Job)
}

func NewJobScope(ctx context.Context, client client.Client, job *batchv1.Job) (*JobScope, error) {
	if job == nil {
		return nil, errors.New("failed to create new job scope, got nil job")
	}

	helper, err := patch.NewHelper(job, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create new pipeline scope, go error: %w", err)
	}

	return &JobScope{
		Client:      client,
		Job:         job,
		ctx:         ctx,
		patchHelper: helper,
	}, nil
}
