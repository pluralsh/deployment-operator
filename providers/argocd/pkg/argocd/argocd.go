package argocd

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Argocd struct {
	apiclient.Client
}

func (c *Argocd) CreateApplication(ctx context.Context, name string) (*v1alpha1.Application, error) {
	closer, appClient, err := c.NewApplicationClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return appClient.Create(context.Background(), &applicationpkg.ApplicationCreateRequest{
		Application: &v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1alpha1.ApplicationSpec{
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "",
					Path:           "",
					TargetRevision: "",
					Helm:           nil,
					Kustomize:      nil,
					Directory:      nil,
					Plugin:         nil,
					Chart:          "",
					Ref:            "",
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "",
					Namespace: "",
					Name:      "",
				},
				Project:              "",
				SyncPolicy:           &v1alpha1.SyncPolicy{},
				IgnoreDifferences:    nil,
				Info:                 nil,
				RevisionHistoryLimit: nil,
				Sources:              v1alpha1.ApplicationSources{},
			},
			Status: v1alpha1.ApplicationStatus{},
			Operation: &v1alpha1.Operation{
				Sync:        nil,
				InitiatedBy: v1alpha1.OperationInitiator{},
				Info:        nil,
				Retry:       v1alpha1.RetryStrategy{},
			},
		},
		Upsert:   nil,
		Validate: nil,
	})
}

func (c *Argocd) DeleteApplication(ctx context.Context, namespace, name, project string) (err error) {
	closer, appClient, err := c.NewApplicationClient()
	if err != nil {
		return err
	}
	defer closer.Close()

	flagTrue := true
	appClient.Delete(ctx, &applicationpkg.ApplicationDeleteRequest{
		Name:              &name,
		Cascade:           &flagTrue,
		PropagationPolicy: nil,
		AppNamespace:      &namespace,
		Project:           &project,
	})
	return nil
}

func (c *Argocd) GetApplicationStatus(ctx context.Context, name string) (*v1alpha1.ApplicationStatus, error) {
	closer, appClient, err := c.NewApplicationClient()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	app, err := appClient.Get(ctx, &applicationpkg.ApplicationQuery{
		Name: &name,
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
