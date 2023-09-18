package provisioner

import (
	"context"
	"net"
	"net/url"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	deploymentspec "github.com/pluralsh/deployment-api/spec"
	"k8s.io/klog/v2"
)

type ProvisionerServer struct {
	address           string
	identityServer    deploymentspec.IdentityServer
	provisionerServer deploymentspec.ProvisionerServer

	listenOpts []grpc.ServerOption
}

func (s *ProvisionerServer) Run(ctx context.Context) error {
	addr, err := url.Parse(s.address)
	if err != nil {
		return err
	}

	if addr.Scheme != "unix" {
		err := errors.New("Address must be a unix domain socket")
		klog.ErrorS(err, "Unsupported scheme", "expected", "unix", "found", addr.Scheme)
		return errors.Wrap(err, "Invalid argument")
	}

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(ctx, "unix", addr.Path)
	if err != nil {
		klog.ErrorS(err, "Failed to start server")
		return errors.Wrap(err, "Failed to start server")
	}

	server := grpc.NewServer(s.listenOpts...)

	if s.provisionerServer == nil || s.identityServer == nil {
		err := errors.New("ProvisionerServer and identity server cannot be nil")
		klog.ErrorS(err, "Invalid args")
		return errors.Wrap(err, "Invalid args")
	}

	deploymentspec.RegisterIdentityServer(server, s.identityServer)
	deploymentspec.RegisterProvisionerServer(server, s.provisionerServer)

	errChan := make(chan error)
	go func() {
		errChan <- server.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		server.GracefulStop()
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}
