package provisioner

import (
	"context"
	"net"
	"net/url"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pluralsh/deployment-operator/common/log"
	proto "github.com/pluralsh/deployment-operator/provisioner/proto"
)

type Server struct {
	address           string
	identityServer    proto.IdentityServer
	provisionerServer proto.ProvisionerServer

	listenOpts []grpc.ServerOption
}

func (s *Server) Run(ctx context.Context) error {
	addr, err := url.Parse(s.address)
	if err != nil {
		return err
	}

	if addr.Scheme != "unix" {
		msg := "Unsupported scheme: Address must be a unix domain socket"
		log.Logger.Errorw(msg, "expected", "unix", "found", addr.Scheme)
		return errors.Wrap(err, "Invalid argument")
	}

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(ctx, "unix", addr.Path)
	if err != nil {
		log.Logger.Error(err, "Failed to start server")
		return errors.Wrap(err, "Failed to start server")
	}

	server := grpc.NewServer(s.listenOpts...)

	if s.provisionerServer == nil || s.identityServer == nil {
		err := errors.New("ProvisionerServer and identity server cannot be nil")
		log.Logger.Error(err, "Invalid args")
		return errors.Wrap(err, "Invalid args")
	}

	proto.RegisterIdentityServer(server, s.identityServer)
	proto.RegisterProvisionerServer(server, s.provisionerServer)

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
