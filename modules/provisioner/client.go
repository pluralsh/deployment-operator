package provisioner

import (
	"context"

	"google.golang.org/grpc"

	proto "github.com/pluralsh/deployment/provisioner/proto"
)

var (
	_ proto.IdentityClient    = &Client{}
	_ proto.ProvisionerClient = &Client{}
)

type Client struct {
	address           string
	conn              *grpc.ClientConn
	identityClient    proto.IdentityClient
	provisionerClient proto.ProvisionerClient
}

func (c *Client) DriverGetDeploymentStatus(ctx context.Context, in *proto.DriverGetDeploymentStatusRequest, opts ...grpc.CallOption) (*proto.DriverGetDeploymentStatusResponse, error) {
	return c.provisionerClient.DriverGetDeploymentStatus(ctx, in, opts...)
}

func (c *Client) DriverGetInfo(ctx context.Context,
	in *proto.DriverGetInfoRequest,
	opts ...grpc.CallOption) (*proto.DriverGetInfoResponse, error) {

	return c.identityClient.DriverGetInfo(ctx, in, opts...)
}

func (c *Client) DriverCreateDeployment(ctx context.Context,
	in *proto.DriverCreateDeploymentRequest,
	opts ...grpc.CallOption) (*proto.DriverCreateDeploymentResponse, error) {

	return c.provisionerClient.DriverCreateDeployment(ctx, in, opts...)
}

func (c *Client) DriverDeleteDeployment(ctx context.Context,
	in *proto.DriverDeleteDeploymentRequest,
	opts ...grpc.CallOption) (*proto.DriverDeleteDeploymentResponse, error) {

	return c.provisionerClient.DriverDeleteDeployment(ctx, in, opts...)
}
