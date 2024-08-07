package controller

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/client"
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

	ctx context.Context

	client client.Client

	Socket *websocket.Socket
}

func NewControllerManager(ctx context.Context, maxConcurrentReconciles int, cacheSyncTimeout time.Duration,
	refresh, jitter time.Duration, recoverPanic *bool, consoleUrl, deployToken, clusterId string) (*ControllerManager, error) {

	socket, err := websocket.New(clusterId, consoleUrl, deployToken)
	if err != nil {
		if socket == nil {
			return nil, err
		}
		klog.Error(err, "could not initiate websocket connection, ignoring and falling back to polling")
	}

	return &ControllerManager{
		Controllers:             make([]*Controller, 0),
		MaxConcurrentReconciles: maxConcurrentReconciles,
		CacheSyncTimeout:        cacheSyncTimeout,
		RecoverPanic:            recoverPanic,
		Refresh:                 refresh,
		Jitter:                  jitter,
		started:                 false,
		ctx:                     ctx,
		client:                  client.New(consoleUrl, deployToken),
		Socket:                  socket,
	}, nil
}

func (cm *ControllerManager) GetClient() client.Client {
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
				return controller.Do.Poll(cm.ctx)
			})
		}()

		go func() {
			controller.Start(cm.ctx)
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
