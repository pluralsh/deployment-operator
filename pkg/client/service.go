package client

import (
	console "github.com/pluralsh/console-client-go"
)

const GetServiceDeploymentDocument = `
query GetService($id: ID!) {
	serviceDeployment(id: $id) {
		id
		name
		namespace
		version
		tarball
		deletedAt
		dryRun
		cluster {
			id
			name
			handle
			self
			version
			pingedAt
			currentVersion
			kasUrl
		}
		kustomize {
			path
		}
		helm {
			valuesFiles
		}
		configuration {
			name
			value
		}
	}
}`

type GetService struct {
	ServiceDeployment ServiceDeployment `json:"serviceDeployment" graphql:"serviceDeployment"`
}

type ServiceDeployment struct {
	ID        string  `json:"id" graphql:"id"`
	Name      string  `json:"name" graphql:"name"`
	Namespace string  `json:"namespace" graphql:"namespace"`
	Version   string  `json:"version" graphql:"version"`
	Tarball   *string `json:"tarball" graphql:"tarball"`
	DeletedAt *string `json:"deletedAt" graphql:"deletedAt"`
	DryRun    *bool   `json:"dryRun" graphql:"dryRun"`

	Cluster       *Cluster   `json:"cluster" graphql:"cluster"`
	Kustomize     *Kustomize `json:"kustomize" graphql:"kustomize"`
	Helm          *Helm      `json:"helm" graphql:"helm"`
	Configuration []*struct {
		Name  string `json:"name" graphql:"name"`
		Value string `json:"value" graphql:"value"`
	} `json:"configuration" graphql:"configuration"`
}

type Cluster struct {
	ID             string  `json:"id" graphql:"id"`
	Name           string  `json:"name" graphql:"name"`
	Handle         *string `json:"handle" graphql:"handle"`
	Self           *bool   `json:"self" graphql:"self"`
	Version        *string `json:"version" graphql:"version"`
	PingedAt       *string `json:"pingedAt" graphql:"pingedAt"`
	CurrentVersion *string `json:"currentVersion" graphql:"currentVersion"`
	KasURL         *string `json:"kasUrl" graphql:"kasUrl"`
}

type Helm struct {
	ValuesFiles []*string `json:"valuesFiles" graphql:"valuesFiles"`
}

type Kustomize struct {
	Path string `json:"path" graphql:"path"`
}

func (c *client) GetServices() ([]*console.ServiceDeploymentBaseFragment, error) {
	resp, err := c.consoleClient.ListClusterServices(c.ctx)
	if err != nil {
		return nil, err
	}

	return resp.ClusterServices, nil
}

func (c *client) GetService(id string) (*ServiceDeployment, error) {
	vars := map[string]interface{}{
		"id": id,
	}

	var res GetService
	if err := c.consoleClient.Client.Post(c.ctx, "GetService", GetServiceDeploymentDocument, &res, vars); err != nil {
		return nil, err
	}

	return &res.ServiceDeployment, nil
}

func (c *client) UpdateComponents(id string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes) error {
	_, err := c.consoleClient.UpdateServiceComponents(c.ctx, id, components, errs)
	return err
}

func (c *client) AddServiceErrors(id string, errs []*console.ServiceErrorAttributes) error {
	_, err := c.consoleClient.AddServiceError(c.ctx, id, errs)
	return err
}
