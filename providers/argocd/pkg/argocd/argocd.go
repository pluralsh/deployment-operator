package argocd

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type Argocd struct {
	apiclient.Client
}

func (c *Argocd) CreateApplication(ctx context.Context, application *v1alpha1.Application) (*v1alpha1.Application, error) {
	closer, appClient, err := c.NewApplicationClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	upsert := true
	validate := true
	return appClient.Create(ctx, &applicationpkg.ApplicationCreateRequest{
		Application: application,
		Upsert:      &upsert,
		Validate:    &validate,
	})
}

func (c *Argocd) DeleteApplication(ctx context.Context, namespace, name, project string) (err error) {
	closer, appClient, err := c.NewApplicationClient()
	if err != nil {
		return err
	}
	defer closer.Close()

	flagTrue := true
	_, err = appClient.Delete(ctx, &applicationpkg.ApplicationDeleteRequest{
		Name:              &name,
		Cascade:           &flagTrue,
		PropagationPolicy: nil,
		AppNamespace:      &namespace,
		Project:           &project,
	})
	return err
}

func (c *Argocd) GetApplicationStatus(ctx context.Context, namespace, name, project string) (*v1alpha1.ApplicationStatus, error) {
	closer, appClient, err := c.NewApplicationClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	app, err := appClient.Get(ctx, &applicationpkg.ApplicationQuery{
		Name:         &name,
		Project:      []string{project},
		AppNamespace: &namespace,
	})
	if err != nil {
		return nil, err
	}
	return &app.Status, nil
}

func NewArgocd() (*Argocd, error) {
	// Set the following environment variables
	// ARGOCD_SERVER and ARGOCD_AUTH_TOKEN
	client, err := apiclient.NewClient(&apiclient.ClientOptions{
		Insecure: true,
	})
	if err != nil {
		return nil, err
	}
	return &Argocd{
		client,
	}, nil

}
