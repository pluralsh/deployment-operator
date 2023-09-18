package provisioner

import (
	"context"

	deploymentspec "github.com/pluralsh/deployment-api/spec"
	"google.golang.org/grpc"
)

var (
	_ deploymentspec.IdentityClient    = &ProvisionerClient{}
	_ deploymentspec.ProvisionerClient = &ProvisionerClient{}
)

type ProvisionerClient struct {
	address           string
	conn              *grpc.ClientConn
	identityClient    deploymentspec.IdentityClient
	provisionerClient deploymentspec.ProvisionerClient
}

func (c *ProvisionerClient) DriverGetInfo(ctx context.Context,
	in *deploymentspec.DriverGetInfoRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverGetInfoResponse, error) {

	return c.identityClient.DriverGetInfo(ctx, in, opts...)
}

func (c *ProvisionerClient) DriverCreateDeployment(ctx context.Context,
	in *deploymentspec.DriverCreateDeploymentRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverCreateDeploymentResponse, error) {

	return c.provisionerClient.DriverCreateDeployment(ctx, in, opts...)
}

func (c *ProvisionerClient) DriverDeleteDeployment(ctx context.Context,
	in *deploymentspec.DriverDeleteDeploymentRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverDeleteDeploymentResponse, error) {

	return c.provisionerClient.DriverDeleteDeployment(ctx, in, opts...)
}

func (c *ProvisionerClient) DriverGrantDeploymentAccess(ctx context.Context,
	in *deploymentspec.DriverGrantDeploymentAccessRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverGrantDeploymentAccessResponse, error) {

	return c.provisionerClient.DriverGrantDeploymentAccess(ctx, in, opts...)
}

func (c *ProvisionerClient) DriverRevokeDeploymentAccess(ctx context.Context,
	in *deploymentspec.DriverRevokeDeploymentAccessRequest,
	opts ...grpc.CallOption) (*deploymentspec.DriverRevokeDeploymentAccessResponse, error) {

	return c.provisionerClient.DriverRevokeDeploymentAccess(ctx, in, opts...)
}
