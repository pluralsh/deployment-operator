package provider

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/pluralsh/deployment-operator/common/log"
	deploymentspec "github.com/pluralsh/deployment-operator/provisioner/proto"
)

func NewProvider(provider string) (*IdentityServer, *Server) {
	return &IdentityServer{
			provider: provider,
		}, &Server{
			provider:   provider,
			deployment: map[string]string{},
		}
}

type Server struct {
	provider   string
	deployment map[string]string
}

func (ps *Server) ProviderGetDeploymentStatus(ctx context.Context, request *deploymentspec.ProviderGetDeploymentStatusRequest) (*deploymentspec.ProviderGetDeploymentStatusResponse, error) {
	return &deploymentspec.ProviderGetDeploymentStatusResponse{
		DeploymentId: request.DeploymentId,
		DeploymentStatus: &deploymentspec.DeploymentStatusEnum{
			Type: &deploymentspec.DeploymentStatusEnum_Ready{
				Ready: true,
			},
		},
		Message: "",
	}, nil
}

func (ps *Server) ProviderCreateDeployment(_ context.Context, req *deploymentspec.ProviderCreateDeploymentRequest) (*deploymentspec.ProviderCreateDeploymentResponse, error) {
	deploymentName := req.GetName()
	log.Logger.Infow("Create Deployment", "name", deploymentName)

	if ps.deployment[deploymentName] != "" {
		return &deploymentspec.ProviderCreateDeploymentResponse{}, status.Error(codes.AlreadyExists, "Deployment already exists")
	}
	dbID := MakeDeploymentID()
	ps.deployment[deploymentName] = dbID

	return &deploymentspec.ProviderCreateDeploymentResponse{
		DeploymentId: dbID,
	}, nil
}

func (ps *Server) ProviderDeleteDeployment(_ context.Context, req *deploymentspec.ProviderDeleteDeploymentRequest) (*deploymentspec.ProviderDeleteDeploymentResponse, error) {
	for name, id := range ps.deployment {
		if req.DeploymentId == id {
			delete(ps.deployment, name)
			return &deploymentspec.ProviderDeleteDeploymentResponse{}, nil
		}
	}
	return &deploymentspec.ProviderDeleteDeploymentResponse{}, status.Error(codes.NotFound, "Deployment not found")
}

type IdentityServer struct {
	provider string
}

func (id *IdentityServer) ProviderGetInfo(context.Context, *deploymentspec.ProviderGetInfoRequest) (*deploymentspec.ProviderGetInfoResponse, error) {
	if id.provider == "" {
		log.Logger.Error("provider name cannot be empty", "Invalid argument")
		return nil, status.Error(codes.InvalidArgument, "ProviderName is empty")
	}

	return &deploymentspec.ProviderGetInfoResponse{
		Name: id.provider,
	}, nil
}

func MakeDeploymentID() string {
	alpha := "abcdefghijklmnopqrstuvwxyz"
	r := rand.Intn(len(alpha))
	return fmt.Sprintf("%c%s", alpha[r], rand.String(9))
}
