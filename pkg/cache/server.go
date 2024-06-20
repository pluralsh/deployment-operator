package cache

import (
	"context"

	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type watcherWrapper struct {
	w          watcher.StatusWatcher
	id         object.ObjMetadata
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type ServerCache struct {
	resourceToWatcher map[string]*watcherWrapper
}

func NewServerCache() *ServerCache {
	return &ServerCache{
		resourceToWatcher: make(map[string]*watcherWrapper),
	}
}
