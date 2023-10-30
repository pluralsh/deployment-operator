package agent

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/flowcontrol"
)

func disableClientLimits(config *rest.Config) {
	enabled, err := flowcontrol.IsEnabled(context.Background(), config)
	if err != nil {
		log.Error(err, "could not determine if flowcontrol was enabled")
	} else if enabled {
		log.Info("flow control enabled, disabling client side throttling")
		config.QPS = -1
		config.Burst = -1
		config.RateLimiter = nil
	}
}
