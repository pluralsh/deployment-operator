package client

import (
	"context"
	"net/http"

	console "github.com/pluralsh/console/go/client"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/stackrun/v1"
)

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
	return &client{
		consoleClient: console.NewClient(&http.Client{
			Transport: helpers.NewAuthorizationTokenTransport(token),
		}, url, nil),
		ctx:   context.Background(),
		url:   url,
		token: token,
	}
}

type Client interface {
	GetCredentials() (url, token string)
	PingCluster(attributes console.ClusterPing) error
	Ping(vsn string) error
	RegisterRuntimeServices(svcs map[string]string, serviceId *string) error
	UpsertVirtualCluster(parentID string, attributes console.ClusterAttributes) (*console.GetClusterWithToken_Cluster, error)
	IsClusterExists(id string) (bool, error)
	GetCluster(id string) (*console.TinyClusterFragment, error)
	MyCluster() (*console.MyCluster, error)
	DetachCluster(id string) error
	GetClusterRestore(id string) (*console.ClusterRestoreFragment, error)
	UpdateClusterRestore(id string, attrs console.RestoreAttributes) (*console.ClusterRestoreFragment, error)
	SaveClusterBackup(attrs console.BackupAttributes) (*console.ClusterBackupFragment, error)
	GetClusterBackup(clusterID, namespace, name string) (*console.ClusterBackupFragment, error)
	GetServices(after *string, first *int64) (*console.PagedClusterServiceIds, error)
	GetService(id string) (*console.ServiceDeploymentForAgent, error)
	GetServiceDeploymentComponents(id string) (*console.GetServiceDeploymentComponents_ServiceDeployment, error)
	UpdateComponents(id, revisionID string, sha *string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes) error
	AddServiceErrors(id string, errs []*console.ServiceErrorAttributes) error
	ParsePipelineGateCR(pgFragment *console.PipelineGateFragment, operatorNamespace string) (*v1alpha1.PipelineGate, error)
	GateExists(id string) bool
	GetClusterGate(id string) (*console.PipelineGateFragment, error)
	GetClusterGates(after *string, first *int64) (*console.PagedClusterGateIDs, error)
	UpdateGate(id string, attributes console.GateUpdateAttributes) error
	UpsertConstraints(constraints []*console.PolicyConstraintAttributes) (*console.UpsertPolicyConstraints, error)
	GetNamespace(id string) (*console.ManagedNamespaceFragment, error)
	ListNamespaces(after *string, first *int64) (*console.ListClusterNamespaces_ClusterManagedNamespaces, error)
	GetStackRunBase(id string) (*v1.StackRun, error)
	GetStackRun(id string) (*console.StackRunMinimalFragment, error)
	AddStackRunLogs(id, logs string) error
	CompleteStackRun(id string, attributes console.StackRunAttributes) error
	UpdateStackRun(id string, attributes console.StackRunAttributes) error
	UpdateStackRunStep(id string, attributes console.RunStepAttributes) error
	ListClusterStackRuns(after *string, first *int64) (*console.ListClusterStackIds_ClusterStackRuns, error)
	GetUser(email string) (*console.UserFragment, error)
	GetGroup(name string) (*console.GroupFragment, error)
	SaveUpgradeInsights(attributes []*console.UpgradeInsightAttributes, addons []*console.CloudAddonAttributes) (*console.SaveUpgradeInsights, error)
	UpsertVulnerabilityReports(vulnerabilities []*console.VulnerabilityReportAttributes) (*console.UpsertVulnerabilities, error)
	IngestClusterCost(attr console.CostIngestAttributes) (*console.IngestClusterCost, error)
}
