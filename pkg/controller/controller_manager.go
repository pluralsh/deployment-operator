package controller

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
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

	Jitter time.Duration

	// started is true if the ControllerManager has been Started
	started bool

	client client.Client

	Socket *websocket.Socket
}

func NewControllerManager(options ...ControllerManagerOption) (*ControllerManager, error) {
	ctrl := &ControllerManager{
		Controllers: make([]*Controller, 0),
		started:     false,
	}

	for _, option := range options {
		if err := option(ctrl); err != nil {
			return nil, err
		}
	}

	return ctrl, nil
}

func (cm *ControllerManager) GetClient() client.Client {
	return cm.client
}

func (cm *ControllerManager) AddController(ctrl *Controller) {
	ctrl.SetupWithManager(cm)

	cm.Controllers = append(cm.Controllers, ctrl)
}

func (cm *ControllerManager) GetReconciler(name string) Reconciler {
	for _, ctrl := range cm.Controllers {
		if ctrl.Name == name {
			return ctrl.Do
		}
	}

	return nil
}

func (cm *ControllerManager) AddReconcilerOrDie(name string, reconcilerGetter func() (Reconciler, workqueue.RateLimitingInterface, error)) {
	reconciler, queue, err := reconcilerGetter()
	if err != nil {
		log.Logger.Errorw("unable to create reconciler", "name", name, "error", err)
		os.Exit(1)
	}

	cm.AddController(&Controller{
		Name:  name,
		Do:    reconciler,
		Queue: queue,
	})
}

func (cm *ControllerManager) Start(ctx context.Context) error {
	if cm.started {
		return errors.New("controller manager was started more than once")
	}

	for _, ctrl := range cm.Controllers {
		controller := ctrl
		jitterValue := time.Duration(rand.Int63n(int64(cm.Jitter)))
		cm.Socket.AddPublisher(controller.Do.GetPublisher())

		go func() {
			defer controller.Do.ShutdownQueue()
			defer controller.Do.WipeCache()

			pollInterval := cm.Refresh
			if controllerPollInterval := controller.Do.GetPollInterval(); controllerPollInterval > 0 {
				pollInterval = controllerPollInterval
			}
			pollInterval += jitterValue
			_ = wait.PollUntilContextCancel(context.Background(), pollInterval, true, func(_ context.Context) (done bool, err error) {
				return controller.Do.Poll(ctx)
			})
		}()

		go func() {
			controller.Start(ctx)
		}()
	}

	go func() {
		_ = wait.PollUntilContextCancel(context.Background(), cm.Refresh, true, func(_ context.Context) (done bool, err error) {
			return false, cm.Socket.Join()
		})
	}()

	cm.started = true
	return nil
}
