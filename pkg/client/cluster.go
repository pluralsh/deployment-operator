package client

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	internalerrors "github.com/pluralsh/deployment-operator/internal/errors"
)

const (
	certManagerServiceName  = "cert-manager"
	ebsCsiDriverServiceName = "aws-ebs-csi-driver"
	externalDNSServiceName  = "external-dns"
	linkerdServiceName      = "Linkerd"
	istioServiceName        = "istio"
	ciliumServiceName       = "cilium"
)

func (c *client) PingCluster(attributes console.ClusterPing) error {
	_, err := c.consoleClient.PingCluster(c.ctx, attributes)
	return err
}

func (c *client) Ping(vsn string) error {
	_, err := c.consoleClient.PingCluster(c.ctx, console.ClusterPing{CurrentVersion: vsn})
	return err
}

func initLayouts(layouts *console.OperationalLayoutAttributes) *console.OperationalLayoutAttributes {
	if layouts == nil {
		return &console.OperationalLayoutAttributes{
			Namespaces: &console.ClusterNamespacesAttributes{},
		}
	}
	return layouts
}

func appendUniqueExternalDNSNamespace(slice []*string, newValue *string) []*string {
	if slice == nil {
		slice = make([]*string, 0)
	}
	sliceSet := containers.ToSet[*string](slice)
	sliceSet.Add(newValue)
	return sliceSet.List()
}

func (c *client) RegisterRuntimeServices(svcs map[string]*NamespaceVersion, serviceId *string, serviceMesh *console.ServiceMesh) error {
	inputs := make([]*console.RuntimeServiceAttributes, 0)
	var layouts *console.OperationalLayoutAttributes
	for name, nv := range svcs {
		inputs = append(inputs, &console.RuntimeServiceAttributes{
			Name:    name,
			Version: nv.Version,
		})
		switch name {
		case certManagerServiceName:
			layouts = initLayouts(layouts)
			layouts.Namespaces.CertManager = &nv.Namespace
		case ebsCsiDriverServiceName:
			layouts = initLayouts(layouts)
			layouts.Namespaces.EbsCsiDriver = &nv.Namespace
		case externalDNSServiceName:
			layouts = initLayouts(layouts)
			layouts.Namespaces.ExternalDNS = appendUniqueExternalDNSNamespace(layouts.Namespaces.ExternalDNS, &nv.Namespace)
		}
		if nv.PartOf != "" {
			switch nv.PartOf {
			case linkerdServiceName:
				layouts = initLayouts(layouts)
				layouts.Namespaces.Linkerd = &nv.Namespace
			case istioServiceName:
				layouts = initLayouts(layouts)
				layouts.Namespaces.Istio = &nv.Namespace
			case ciliumServiceName:
				layouts = initLayouts(layouts)
				layouts.Namespaces.Cilium = &nv.Namespace
			}
		}
	}

	if layouts == nil && serviceMesh != nil {
		layouts = &console.OperationalLayoutAttributes{}
	}

	layouts.ServiceMesh = serviceMesh
	_, err := c.consoleClient.RegisterRuntimeServices(c.ctx, inputs, layouts, serviceId)
	return err
}

func (c *client) MyCluster() (*console.MyCluster, error) {
	return c.consoleClient.MyCluster(c.ctx)
}

func (c *client) UpsertVirtualCluster(parentID string, attributes console.ClusterAttributes) (*console.GetClusterWithToken_Cluster, error) {
	cluster, err := c.consoleClient.UpsertVirtualCluster(c.ctx, parentID, attributes)
	if err != nil {
		return nil, err
	}

	if cluster == nil {
		return nil, nil
	}

	return &console.GetClusterWithToken_Cluster{
		DeployToken: cluster.UpsertVirtualCluster.DeployToken,
		ID:          cluster.UpsertVirtualCluster.ID,
		Name:        cluster.UpsertVirtualCluster.Name,
		Handle:      cluster.UpsertVirtualCluster.Handle,
		Self:        cluster.UpsertVirtualCluster.Self,
		Project:     cluster.UpsertVirtualCluster.Project,
	}, nil
}

func (c *client) IsClusterExists(id string) (bool, error) {
	cluster, err := c.GetCluster(id)
	if errors.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return cluster != nil, nil
}

func (c *client) GetCluster(id string) (*console.TinyClusterFragment, error) {
	cluster, err := c.consoleClient.GetCluster(c.ctx, &id)
	if internalerrors.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if internalerrors.IsNotFound(err) || cluster == nil || cluster.Cluster == nil {
		return nil, errors.NewNotFound(schema.GroupResource{}, id)
	}

	return &console.TinyClusterFragment{
		ID:      cluster.Cluster.ID,
		Name:    cluster.Cluster.Name,
		Handle:  cluster.Cluster.Handle,
		Self:    cluster.Cluster.Self,
		Project: cluster.Cluster.Project,
	}, nil
}

func (c *client) DetachCluster(id string) error {
	_, err := c.consoleClient.DetachCluster(c.ctx, id)
	return err
}
