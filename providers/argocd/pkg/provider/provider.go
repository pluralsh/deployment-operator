package provider

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pluralsh/deployment-operator/providers/argocd/pkg/argocd"
	proto "github.com/pluralsh/deployment-operator/provisioner/proto"
)

func NewProvider(provisioner string) (*IdentityServer, *Server, error) {
	argocdProvider, err := argocd.NewArgocd()
	if err != nil {
		return nil, nil, err
	}
	return &IdentityServer{
			provisioner: provisioner,
		}, &Server{
			provisioner: provisioner,
			argocd:      argocdProvider,
		}, nil
}

type Server struct {
	provisioner string
	argocd      *argocd.Argocd
}

func (ps *Server) ProviderGetDeploymentStatus(ctx context.Context, request *proto.ProviderGetDeploymentStatusRequest) (*proto.ProviderGetDeploymentStatusResponse, error) {
	return &proto.ProviderGetDeploymentStatusResponse{
		DeploymentId: request.DeploymentId,
		DeploymentStatus: &proto.DeploymentStatusEnum{
			Type: &proto.DeploymentStatusEnum_Ready{
				Ready: true,
			},
		},
		Message: "",
	}, nil
}

func (ps *Server) ProviderCreateDeployment(_ context.Context, req *proto.ProviderCreateDeploymentRequest) (*proto.ProviderCreateDeploymentResponse, error) {
	deploymentName := req.GetName()

	dbID := deploymentName

	return &proto.ProviderCreateDeploymentResponse{
		DeploymentId: dbID,
	}, nil
}

func (ps *Server) ProviderDeleteDeployment(_ context.Context, req *proto.ProviderDeleteDeploymentRequest) (*proto.ProviderDeleteDeploymentResponse, error) {

	return &proto.ProviderDeleteDeploymentResponse{}, status.Error(codes.NotFound, "Deployment not found")
}

type IdentityServer struct {
	provisioner string
}

func (id *IdentityServer) ProviderGetInfo(context.Context, *proto.ProviderGetInfoRequest) (*proto.ProviderGetInfoResponse, error) {
	if id.provisioner == "" {
		return nil, status.Error(codes.InvalidArgument, "ProviderName is empty")
	}

	return &proto.ProviderGetInfoResponse{
		Name: id.provisioner,
	}, nil
}
