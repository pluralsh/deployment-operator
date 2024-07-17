// Copyright 2022 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
)

// Options can be provided when creating a new StatusWatcher to customize the
// behavior.
type Options struct {
	// RESTScopeStrategy specifies which strategy to use when listing and
	// watching resources. By default, the strategy is selected automatically.
	RESTScopeStrategy *kwatcher.RESTScopeStrategy

	// ObjectFilter is used to filter resources after getting them from the API.
	ObjectFilter kwatcher.ObjectFilter

	// UseCustomObjectFilter controls whether custom ObjectFilter provided in options
	// should be used instead of the default one.
	UseCustomObjectFilter bool

	// Filters allows filtering the objects being watched.
	Filters *kwatcher.Filters

	// UseInformerRefCache allows caching informer ref per status watcher instance.
	// This allows to ensure that multiple [StatusWatcher.Watch] calls will only spawn
	// unique watches.
	UseInformerRefCache bool

	ID string
}
