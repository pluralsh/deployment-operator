package controller

import (
	"time"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

type ControllerManagerOption func(*ControllerManager) error

func WithConsoleClient(client client.Client) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.client = client
		return nil
	}
}

func WithConsoleClientArgs(url string, deployToken string) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.client = client.New(url, deployToken)
		return nil
	}
}

func WithSocket(socket *websocket.Socket) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.Socket = socket
		return nil
	}
}

func WithSocketArgs(clusterID, url, deployToken string) ControllerManagerOption {
	return func(o *ControllerManager) (err error) {
		socket, err := websocket.New(clusterID, url, deployToken)
		o.Socket = socket

		if err != nil && socket != nil {
			klog.Error(err, "could not initiate websocket connection, ignoring and falling back to polling")
			return nil
		}

		return err
	}
}

func WithMaxConcurrentReconciles(maxConcurrentReconciles int) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.MaxConcurrentReconciles = maxConcurrentReconciles
		return nil
	}
}

func WithCacheSyncTimeout(timeout time.Duration) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.CacheSyncTimeout = timeout
		return nil
	}
}

func WithRefresh(refresh time.Duration) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.Refresh = refresh
		return nil
	}
}

func WithJitter(jitter time.Duration) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.Jitter = jitter
		return nil
	}
}

func WithRecoverPanic(recoverPanic bool) ControllerManagerOption {
	return func(o *ControllerManager) error {
		o.RecoverPanic = &recoverPanic
		return nil
	}
}
