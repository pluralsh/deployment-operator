package driver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pluralsh/deployment-operator/argocd-driver/pkg/argocd"
	proto "github.com/pluralsh/deployment-operator/provisioner/proto"
)

func NewDriver(provisioner string) (*IdentityServer, *ProvisionerServer) {
	return &IdentityServer{
			provisioner: provisioner,
		}, &ProvisionerServer{
			provisioner: provisioner,
		}
}

type ProvisionerServer struct {
	provisioner string
	argocd      *argocd.Argocd
}

func (ps *ProvisionerServer) DriverGetDeploymentStatus(ctx context.Context, request *proto.DriverGetDeploymentStatusRequest) (*proto.DriverGetDeploymentStatusResponse, error) {
	return &proto.DriverGetDeploymentStatusResponse{
		DeploymentId: request.DeploymentId,
		DeploymentStatus: &proto.DeploymentStatusEnum{
			Type: &proto.DeploymentStatusEnum_Ready{
				Ready: true,
			},
		},
		Message: "",
	}, nil
}

func (ps *ProvisionerServer) DriverCreateDeployment(_ context.Context, req *proto.DriverCreateDeploymentRequest) (*proto.DriverCreateDeploymentResponse, error) {
	deploymentName := req.GetName()

	dbID := deploymentName

	return &proto.DriverCreateDeploymentResponse{
		DeploymentId: dbID,
	}, nil
}

func (ps *ProvisionerServer) DriverDeleteDeployment(_ context.Context, req *proto.DriverDeleteDeploymentRequest) (*proto.DriverDeleteDeploymentResponse, error) {

	return &proto.DriverDeleteDeploymentResponse{}, status.Error(codes.NotFound, "Deployment not found")
}

type IdentityServer struct {
	provisioner string
}

func (id *IdentityServer) DriverGetInfo(context.Context, *proto.DriverGetInfoRequest) (*proto.DriverGetInfoResponse, error) {
	if id.provisioner == "" {
		return nil, status.Error(codes.InvalidArgument, "ProvisionerName is empty")
	}

	return &proto.DriverGetInfoResponse{
		Name: id.provisioner,
	}, nil
}
