package controller

import (
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	DefaultPageSize = int64(100)
)

var NamespaceGVK = schema.GroupVersionKind{
	Group: "", Version: "v1", Kind: "Namespace",
}

var ClusterResources = containers.ToSet[schema.GroupVersionKind]([]schema.GroupVersionKind{
	{Group: "", Version: "v1", Kind: "Node"},
	{Group: "", Version: "v1", Kind: "PersistentVolume"},
	NamespaceGVK,
	{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"},
	{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"},
	{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"},
	{Group: "apiregistration.k8s.io", Version: "v1", Kind: "APIService"},
	{Group: "scheduling.k8s.io", Version: "v1", Kind: "PriorityClass"},
})
