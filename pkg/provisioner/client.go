package provisioner

import (
	"context"

	deploymentspec "github.com/pluralsh/deployment-api/spec"
	"google.golang.org/grpc"
)

var (
	_ deploymentspec.IdentityClient    = &Client{}
	_ deploymentspec.ProvisionerClient = &Client{}
)

type Client struct {
	address           string
	conn              *grpc.ClientConn
	identityClient    deploymentspec.IdentityClient
	provisionerClient deploymentspec.ProvisionerClient
}

func (c *Client) DriverGetInfo(ctx context.Context,
	in *deploymentspec.DriverGetInfoRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverGetInfoResponse, error) {

	return c.identityClient.DriverGetInfo(ctx, in, opts...)
}

func (c *Client) DriverCreateDeployment(ctx context.Context,
	in *deploymentspec.DriverCreateDeploymentRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverCreateDeploymentResponse, error) {

	return c.provisionerClient.DriverCreateDeployment(ctx, in, opts...)
}

func (c *Client) DriverDeleteDeployment(ctx context.Context,
	in *deploymentspec.DriverDeleteDeploymentRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverDeleteDeploymentResponse, error) {

	return c.provisionerClient.DriverDeleteDeployment(ctx, in, opts...)
}

func (c *Client) DriverGrantDeploymentAccess(ctx context.Context,
	in *deploymentspec.DriverGrantDeploymentAccessRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverGrantDeploymentAccessResponse, error) {

	return c.provisionerClient.DriverGrantDeploymentAccess(ctx, in, opts...)
}

func (c *Client) DriverRevokeDeploymentAccess(ctx context.Context,
	in *deploymentspec.DriverRevokeDeploymentAccessRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverRevokeDeploymentAccessResponse, error) {

	return c.provisionerClient.DriverRevokeDeploymentAccess(ctx, in, opts...)
}
