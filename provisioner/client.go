package provisioner

import (
	"context"

	"google.golang.org/grpc"

	proto "github.com/pluralsh/deployment-operator/provisioner/proto"
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

func (c *Client) ProviderGetDeploymentStatus(ctx context.Context, in *proto.ProviderGetDeploymentStatusRequest, opts ...grpc.CallOption) (*proto.ProviderGetDeploymentStatusResponse, error) {
	return c.provisionerClient.ProviderGetDeploymentStatus(ctx, in, opts...)
}

func (c *Client) ProviderGetInfo(ctx context.Context,
	in *proto.ProviderGetInfoRequest,
	opts ...grpc.CallOption) (*proto.ProviderGetInfoResponse, error) {

	return c.identityClient.ProviderGetInfo(ctx, in, opts...)
}

func (c *Client) ProviderCreateDeployment(ctx context.Context,
	in *proto.ProviderCreateDeploymentRequest,
	opts ...grpc.CallOption) (*proto.ProviderCreateDeploymentResponse, error) {

	return c.provisionerClient.ProviderCreateDeployment(ctx, in, opts...)
}

func (c *Client) ProviderDeleteDeployment(ctx context.Context,
	in *proto.ProviderDeleteDeploymentRequest,
	opts ...grpc.CallOption) (*proto.ProviderDeleteDeploymentResponse, error) {

	return c.provisionerClient.ProviderDeleteDeployment(ctx, in, opts...)
}
