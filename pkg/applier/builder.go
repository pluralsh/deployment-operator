// Copyright 2021 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/apply/info"
	"sigs.k8s.io/cli-utils/pkg/apply/prune"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/clusterreader"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/statusreaders"
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"

	"github.com/pluralsh/deployment-operator/pkg/common"
)

type commonBuilder struct {
	// factory is only used to retrieve things that have not been provided explicitly.
	factory                      util.Factory
	invClient                    inventory.Client
	client                       dynamic.Interface
	discoClient                  discovery.CachedDiscoveryInterface
	mapper                       meta.RESTMapper
	restConfig                   *rest.Config
	unstructuredClientForMapping func(*meta.RESTMapping) (resource.RESTClient, error)
	statusWatcher                kwatcher.StatusWatcher
}

func (cb *commonBuilder) finalize() (*commonBuilder, error) {
	cx := *cb // make a copy before mutating any fields. Shallow copy is good enough.
	var err error
	if cx.invClient == nil {
		return nil, errors.New("inventory client must be provided")
	}
	if cx.client == nil {
		if cx.factory == nil {
			return nil, fmt.Errorf("a factory must be provided or all other options: %w", err)
		}
		cx.client, err = cx.factory.DynamicClient()
		if err != nil {
			return nil, fmt.Errorf("error getting dynamic client: %w", err)
		}
	}
	if cx.discoClient == nil {
		if cx.factory == nil {
			return nil, fmt.Errorf("a factory must be provided or all other options: %w", err)
		}
		cx.discoClient, err = cx.factory.ToDiscoveryClient()
		if err != nil {
			return nil, fmt.Errorf("error getting discovery client: %w", err)
		}
	}
	if cx.mapper == nil {
		if cx.factory == nil {
			return nil, fmt.Errorf("a factory must be provided or all other options: %w", err)
		}
		cx.mapper, err = cx.factory.ToRESTMapper()
		if err != nil {
			return nil, fmt.Errorf("error getting rest mapper: %w", err)
		}
	}
	if cx.restConfig == nil {
		if cx.factory == nil {
			return nil, fmt.Errorf("a factory must be provided or all other options: %w", err)
		}
		cx.restConfig, err = cx.factory.ToRESTConfig()
		if err != nil {
			return nil, fmt.Errorf("error getting rest config: %w", err)
		}
	}
	if cx.unstructuredClientForMapping == nil {
		if cx.factory == nil {
			return nil, fmt.Errorf("a factory must be provided or all other options: %w", err)
		}
		cx.unstructuredClientForMapping = cx.factory.UnstructuredClientForMapping
	}
	if cx.statusWatcher == nil {
		//cx.statusWatcher = watcher.NewDynamicStatusWatcher(cx.client, cx.discoClient, cx.mapper, watcher.Options{
		//	RESTScopeStrategy: lo.ToPtr(kwatcher.RESTScopeRoot),
		//	Filters: &kwatcher.Filters{
		//		Labels: common.ManagedByAgentLabelSelector(),
		//		Fields: nil,
		//	},
		//})

		cx.statusWatcher = &kwatcher.DefaultStatusWatcher{
			DynamicClient: cx.client,
			Mapper:        cx.mapper,
			ResyncPeriod:  1 * time.Hour,
			StatusReader:  statusreaders.NewDefaultStatusReader(cx.mapper),
			ClusterReader: &clusterreader.DynamicClusterReader{
				DynamicClient: cx.client,
				Mapper:        cx.mapper,
			},
			Indexers: kwatcher.DefaultIndexers(),
			Filters: &kwatcher.Filters{
				Labels: common.ManagedByAgentLabelSelector(),
				Fields: nil,
			},
		}
	}
	return &cx, nil
}

type ApplierBuilder struct {
	commonBuilder
}

// NewApplierBuilder returns a new ApplierBuilder.
func NewApplierBuilder() *ApplierBuilder {
	return &ApplierBuilder{
		// Defaults, if any, go here.
	}
}

func (b *ApplierBuilder) Build() (*Applier, error) {
	bx, err := b.finalize()
	if err != nil {
		return nil, err
	}
	return &Applier{
		pruner: &prune.Pruner{
			InvClient: bx.invClient,
			Client:    bx.client,
			Mapper:    bx.mapper,
		},
		statusWatcher: bx.statusWatcher,
		invClient:     bx.invClient,
		client:        bx.client,
		openAPIGetter: bx.discoClient,
		mapper:        bx.mapper,
		infoHelper:    info.NewHelper(bx.mapper, bx.unstructuredClientForMapping),
	}, nil
}

func (b *ApplierBuilder) WithFactory(factory util.Factory) *ApplierBuilder {
	b.factory = factory
	return b
}

func (b *ApplierBuilder) WithInventoryClient(invClient inventory.Client) *ApplierBuilder {
	b.invClient = invClient
	return b
}

func (b *ApplierBuilder) WithDynamicClient(client dynamic.Interface) *ApplierBuilder {
	b.client = client
	return b
}

func (b *ApplierBuilder) WithDiscoveryClient(discoClient discovery.CachedDiscoveryInterface) *ApplierBuilder {
	b.discoClient = discoClient
	return b
}

func (b *ApplierBuilder) WithRestMapper(mapper meta.RESTMapper) *ApplierBuilder {
	b.mapper = mapper
	return b
}

func (b *ApplierBuilder) WithRestConfig(restConfig *rest.Config) *ApplierBuilder {
	b.restConfig = restConfig
	return b
}

func (b *ApplierBuilder) WithUnstructuredClientForMapping(unstructuredClientForMapping func(*meta.RESTMapping) (resource.RESTClient, error)) *ApplierBuilder {
	b.unstructuredClientForMapping = unstructuredClientForMapping
	return b
}

func (b *ApplierBuilder) WithStatusWatcher(statusWatcher kwatcher.StatusWatcher) *ApplierBuilder {
	b.statusWatcher = statusWatcher
	return b
}
