package provider

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/pluralsh/deployment-operator/providers/argocd/pkg/argocd"
	proto "github.com/pluralsh/deployment-operator/provisioner/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (ps *Server) ProviderCreateDeployment(ctx context.Context, req *proto.ProviderCreateDeploymentRequest) (*proto.ProviderCreateDeploymentResponse, error) {
	deploymentName := req.GetName()
	dbID := deploymentName

	application := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: "argo-cd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        req.Git.Ref,
				Path:           req.Git.Folder,
				TargetRevision: req.Revision.Version,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: req.Namespace,
				Name:      req.Name,
			},
			Project: "default",
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune:      true,
					SelfHeal:   true,
					AllowEmpty: false,
				},
			},
		},
	}

	if _, err := ps.argocd.CreateApplication(ctx, application); err != nil {
		return nil, status.Error(codes.Internal, "Failed to create application")
	}

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
