package watcher

import (
	"strings"

	"github.com/gobuffalo/flect"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func handleFatalError(err error) <-chan event.Event {
	eventCh := make(chan event.Event)
	go func() {
		defer close(eventCh)
		eventCh <- event.Event{
			Type:  event.ErrorEvent,
			Error: err,
		}
	}()
	return eventCh
}

func autoSelectRESTScopeStrategy(ids object.ObjMetadataSet) kwatcher.RESTScopeStrategy {
	if len(uniqueNamespaces(ids)) > 1 {
		return kwatcher.RESTScopeRoot
	}
	return kwatcher.RESTScopeNamespace
}

func rootScopeGKNs(ids object.ObjMetadataSet) []GroupKindNamespace {
	gks := uniqueGKs(ids)
	targets := make([]GroupKindNamespace, len(gks))
	for i, gk := range gks {
		targets[i] = GroupKindNamespace{
			Group:     gk.Group,
			Kind:      gk.Kind,
			Namespace: "",
		}
	}
	return targets
}

func namespaceScopeGKNs(ids object.ObjMetadataSet) []GroupKindNamespace {
	return uniqueGKNs(ids)
}

// uniqueGKNs returns a set of unique GroupKindNamespaces from a set of object identifiers.
func uniqueGKNs(ids object.ObjMetadataSet) []GroupKindNamespace {
	gknMap := make(map[GroupKindNamespace]struct{})
	for _, id := range ids {
		gkn := GroupKindNamespace{Group: id.GroupKind.Group, Kind: id.GroupKind.Kind, Namespace: id.Namespace}
		gknMap[gkn] = struct{}{}
	}
	gknList := make([]GroupKindNamespace, 0, len(gknMap))
	for gk := range gknMap {
		gknList = append(gknList, gk)
	}
	return gknList
}

// uniqueGKs returns a set of unique GroupKinds from a set of object identifiers.
func uniqueGKs(ids object.ObjMetadataSet) []schema.GroupKind {
	gkMap := make(map[schema.GroupKind]struct{})
	for _, id := range ids {
		gkn := schema.GroupKind{Group: id.GroupKind.Group, Kind: id.GroupKind.Kind}
		gkMap[gkn] = struct{}{}
	}
	gkList := make([]schema.GroupKind, 0, len(gkMap))
	for gk := range gkMap {
		gkList = append(gkList, gk)
	}
	return gkList
}

func uniqueNamespaces(ids object.ObjMetadataSet) []string {
	nsMap := make(map[string]struct{})
	for _, id := range ids {
		nsMap[id.Namespace] = struct{}{}
	}
	nsList := make([]string, 0, len(nsMap))
	for ns := range nsMap {
		nsList = append(nsList, ns)
	}
	return nsList
}

func GvrFromGvk(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: flect.Pluralize(strings.ToLower(gvk.Kind)),
	}
}
