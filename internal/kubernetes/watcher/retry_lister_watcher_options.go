package watcher

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type RetryListerWatcherOption func(*RetryListerWatcher)

func WithListerWatcher(listerWatcher cache.ListerWatcher) RetryListerWatcherOption {
	return func(rlw *RetryListerWatcher) {
		rlw.listerWatcher = listerWatcher
	}
}

func WithListOptions(options v1.ListOptions) RetryListerWatcherOption {
	return func(rlw *RetryListerWatcher) {
		rlw.listOptions = options
	}
}

func WithResourceVersion(resourceVersion string) RetryListerWatcherOption {
	return func(rlw *RetryListerWatcher) {
		rlw.initialResourceVersion = resourceVersion
	}
}

func WithID(id string) RetryListerWatcherOption {
	return func(rlw *RetryListerWatcher) {
		rlw.id = id
	}
}

func WithListWatchFunc(listFunc cache.ListFunc, watchFunc cache.WatchFunc) RetryListerWatcherOption {
	return func(rlw *RetryListerWatcher) {
		rlw.listerWatcher = &cache.ListWatch{
			ListFunc:        listFunc,
			WatchFunc:       watchFunc,
			DisableChunking: false,
		}
	}
}
