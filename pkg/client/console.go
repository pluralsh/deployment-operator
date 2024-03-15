package client

import (
	"context"
	"net/http"
	"sync"

	console "github.com/pluralsh/console-client-go"
	v1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
)

type authedTransport struct {
	token   string
	wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Token "+t.token)
	return t.wrapped.RoundTrip(req)
}

var lock = &sync.Mutex{}
var singleInstance Client

type client struct {
	ctx           context.Context
	consoleClient console.ConsoleClient
	url           string
	token         string
}

func (c *client) GetCredentials() (url, token string) {
	return c.url, c.token
}

func New(url, token string) Client {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if singleInstance == nil {
			httpClient := http.Client{
				Transport: &authedTransport{
					token:   token,
					wrapped: http.DefaultTransport,
				},
			}

			singleInstance = &client{
				consoleClient: console.NewClient(&httpClient, url, nil),
				ctx:           context.Background(),
				url:           url,
				token:         token,
			}
		}
	}

	return singleInstance
}

type Client interface {
	GetCredentials() (url, token string)
	PingCluster(attributes console.ClusterPing) error
	Ping(vsn string) error
	RegisterRuntimeServices(svcs map[string]string, serviceId *string) error
	MyCluster() (*console.MyCluster, error)
	GetClusterRestore(id string) (*console.ClusterRestoreFragment, error)
	UpdateClusterRestore(id string, attrs console.RestoreAttributes) (*console.ClusterRestoreFragment, error)
	SaveClusterBackup(attrs console.BackupAttributes) (*console.ClusterBackupFragment, error)
	GetClusterBackup(clusterID, namespace, name string) (*console.ClusterBackupFragment, error)
	GetServices(after *string, first *int64) (*console.PagedClusterServices, error)
	GetService(id string) (*console.GetServiceDeploymentForAgent_ServiceDeployment, error)
	UpdateComponents(id string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes) error
	AddServiceErrors(id string, errs []*console.ServiceErrorAttributes) error
	ParsePipelineGateCR(pgFragment *console.PipelineGateFragment, operatorNamespace string) (*v1alpha1.PipelineGate, error)
	GateExists(id string) bool
	GetClusterGate(id string) (*console.PipelineGateFragment, error)
	GetClusterGates(after *string, first *int64) (*console.PagedClusterGates, error)
	UpdateGate(id string, attributes console.GateUpdateAttributes) error
	UpsertConstraints(constrains []*console.PolicyConstraintAttributes) (*console.UpsertPolicyConstraints, error)
}
