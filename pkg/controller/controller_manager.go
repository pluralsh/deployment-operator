package controller

import (
	"context"
	"errors"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ControllerManager struct {
	Controllers []*Controller

	// MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run.
	MaxConcurrentReconciles int

	// CacheSyncTimeout refers to the time limit set on waiting for cache to sync
	// Defaults to 2 minutes if not set.
	CacheSyncTimeout time.Duration

	// RecoverPanic indicates whether the panic caused by reconcile should be recovered.
	RecoverPanic *bool

	Refresh time.Duration

	// started is true if the ControllerManager has been Started
	started bool

	ctx context.Context

	client *client.Client
}

func NewControllerManager(ctx context.Context, maxConcurrentReconciles int, cacheSyncTimeout time.Duration,
	refresh time.Duration, recoverPanic *bool, consoleUrl, deployToken string) *ControllerManager {
	return &ControllerManager{
		Controllers:             make([]*Controller, 0),
		MaxConcurrentReconciles: maxConcurrentReconciles,
		CacheSyncTimeout:        cacheSyncTimeout,
		RecoverPanic:            recoverPanic,
		Refresh:                 refresh,
		started:                 false,
		ctx:                     ctx,
		client:                  client.New(consoleUrl, deployToken),
	}
}

func (cm *ControllerManager) GetClient() *client.Client {
	return cm.client
}

func (cm *ControllerManager) AddController(ctrl *Controller) {
	ctrl.SetupWithManager(cm)

	cm.Controllers = append(cm.Controllers, ctrl)
}

func (cm *ControllerManager) Start() error {
	if cm.started {
		return errors.New("controller manager was started more than once")
	}

	for _, ctrl := range cm.Controllers {
		controller := ctrl

		go func() {
			defer controller.Do.ShutdownQueue()
			defer controller.Do.WipeCache()
			_ = wait.PollImmediateInfinite(cm.Refresh, func() (done bool, err error) {
				return controller.Do.Poll(cm.ctx)
			})
		}()

		go func() {
			controller.Start(cm.ctx)
		}()
	}

	cm.started = true
	return nil
}
