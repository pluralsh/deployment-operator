package provisioner

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pluralsh/deployment-operator/common/log"
	proto "github.com/pluralsh/deployment-operator/provisioner/proto"
)

const (
	maxGrpcBackoff  = 5 * 30 * time.Second
	grpcDialTimeout = 30 * time.Second
)

func NewDefaultProvisionerClient(ctx context.Context, address string, debug bool) (*Client, error) {
	backoffConfiguration := backoff.DefaultConfig
	backoffConfiguration.MaxDelay = maxGrpcBackoff

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // strictly restricting to local Unix domain socket
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoffConfiguration,
			MinConnectTimeout: grpcDialTimeout,
		}),
		grpc.WithBlock(), // block until connection succeeds
	}

	var interceptors []grpc.UnaryClientInterceptor
	if debug {
		interceptors = append(interceptors, apiLogger)
	}
	return NewProvisionerClient(ctx, address, dialOpts, interceptors)
}

// NewProvisionerClient creates a new GRPCClient that only supports unix domain sockets
func NewProvisionerClient(ctx context.Context, address string, dialOpts []grpc.DialOption, interceptors []grpc.UnaryClientInterceptor) (*Client, error) {
	addr, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	if addr.Scheme != "unix" {
		msg := fmt.Sprintf("Unsupported scheme: Address must be a unix domain socket")
		log.Logger.Errorw(msg, "expected", "unix", "found", addr.Scheme)
		return nil, errors.Wrap(err, "Invalid argument")
	}

	for _, interceptor := range interceptors {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(interceptor))
	}

	ctx, cancel := context.WithTimeout(ctx, maxGrpcBackoff)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, dialOpts...)
	if err != nil {
		msg := fmt.Sprintf("Connection failed: %s", err)
		log.Logger.Errorw(msg, "address", address)
		return nil, err
	}
	return &Client{
		address:           address,
		conn:              conn,
		identityClient:    proto.NewIdentityClient(conn),
		provisionerClient: proto.NewProvisionerClient(conn),
	}, nil
}

func NewDefaultProvisionerServer(address string,
	identityServer proto.IdentityServer,
	provisionerServer proto.ProvisionerServer) (*Server, error) {

	return NewProvisionerServer(address, identityServer, provisionerServer, []grpc.ServerOption{})
}

func NewProvisionerServer(address string,
	identityServer proto.IdentityServer,
	provisionerServer proto.ProvisionerServer,
	listenOpts []grpc.ServerOption) (*Server, error) {

	if identityServer == nil {
		err := errors.New("Identity server cannot be nil")
		log.Logger.Error(err, "Invalid argument")
		return nil, err
	}
	if provisionerServer == nil {
		err := errors.New("Provisioner server cannot be nil")
		log.Logger.Error(err, "Invalid argument")
		return nil, err
	}

	return &Server{
		address:           address,
		identityServer:    identityServer,
		provisionerServer: provisionerServer,
		listenOpts:        listenOpts,
	}, nil
}
