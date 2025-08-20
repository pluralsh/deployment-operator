package store

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type Entry struct {
	UID       string
	ParentUID string
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	Status    string
	ServiceID string
}

type Store interface {
	Save(obj unstructured.Unstructured) error
	Delete(uid types.UID) error
	GetServiceComponents(serviceID string) ([]Entry, error)
}
