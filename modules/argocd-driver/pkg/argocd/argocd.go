package argocd

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
)

type Argocd struct {
}

func (c *Argocd) CreateApplication(ctx context.Context, name string) (err error) {

	return nil
}

func (c *Argocd) DeleteApplication(ctx context.Context, name string) (err error) {
	//klog.Info("Delete index ", name)
	return nil
}

func (c *Argocd) GetApplicationStatus(ctx context.Context, name string) (err error) {

	return nil
}

func NewArgocdClient() (apiclient.Client, error) {
	return apiclient.NewClient(nil)
}