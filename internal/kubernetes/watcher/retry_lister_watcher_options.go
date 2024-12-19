package watcher

import (
	"k8s.io/client-go/tools/cache"
)

type RetryListerWatcherOption func(*RetryListerWatcher)

func WithListerWatcher(listerWatcher cache.ListerWatcher) RetryListerWatcherOption {
	return func(rlw *RetryListerWatcher) {
		rlw.listerWatcher = listerWatcher
	}
}

func WithListOptions() RetryListerWatcherOption {
	return func(rlw *RetryListerWatcher) {

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
