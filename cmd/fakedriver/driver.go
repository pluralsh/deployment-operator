package main

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/util/rand"

	deploymentspec "github.com/pluralsh/deployment-api/spec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

func NewDriver(provisioner string) (*IdentityServer, *ProvisionerServer) {
	return &IdentityServer{
			provisioner: provisioner,
		}, &ProvisionerServer{
			provisioner: provisioner,
			deployment:  map[string]string{},
		}
}

type ProvisionerServer struct {
	provisioner string
	deployment  map[string]string
}

func (ps *ProvisionerServer) DriverCreateDeployment(_ context.Context, req *deploymentspec.DriverCreateDeploymentRequest) (*deploymentspec.DriverCreateDeploymentResponse, error) {
	deploymentName := req.GetName()
	klog.V(3).InfoS("Create Deployment", "name", deploymentName)

	if ps.deployment[deploymentName] != "" {
		return &deploymentspec.DriverCreateDeploymentResponse{}, status.Error(codes.AlreadyExists, "Deployment already exists")
	}
	dbID := MakeDeploymentID()
	ps.deployment[deploymentName] = dbID

	return &deploymentspec.DriverCreateDeploymentResponse{
		DeploymentId: dbID,
	}, nil
}

func (ps *ProvisionerServer) DriverDeleteDeployment(_ context.Context, req *deploymentspec.DriverDeleteDeploymentRequest) (*deploymentspec.DriverDeleteDeploymentResponse, error) {
	for name, id := range ps.deployment {
		if req.DeploymentId == id {
			delete(ps.deployment, name)
			return &deploymentspec.DriverDeleteDeploymentResponse{}, nil
		}
	}
	return &deploymentspec.DriverDeleteDeploymentResponse{}, status.Error(codes.NotFound, "Deployment not found")
}

// DriverGrantDeploymentAccess call grants access to an account. The account_name in the request shall be used as a unique identifier to create credentials.
// The account_id returned in the response will be used as the unique identifier for deleting this access when calling DriverRevokeDeploymentAccess.
func (ps *ProvisionerServer) DriverGrantDeploymentAccess(context.Context, *deploymentspec.DriverGrantDeploymentAccessRequest) (*deploymentspec.DriverGrantDeploymentAccessResponse, error) {
	resp := &deploymentspec.DriverGrantDeploymentAccessResponse{
		AccountId:   "abc",
		Credentials: map[string]*deploymentspec.CredentialDetails{},
	}
	resp.Credentials["cred"] = &deploymentspec.CredentialDetails{Secrets: map[string]string{"a": "b"}}

	return resp, nil
}

// DriverRevokeDeploymentAccess call revokes all access to a particular deployment from a principal.
func (ps *ProvisionerServer) DriverRevokeDeploymentAccess(context.Context, *deploymentspec.DriverRevokeDeploymentAccessRequest) (*deploymentspec.DriverRevokeDeploymentAccessResponse, error) {
	return &deploymentspec.DriverRevokeDeploymentAccessResponse{}, nil
}

type IdentityServer struct {
	provisioner string
}

func (id *IdentityServer) DriverGetInfo(context.Context, *deploymentspec.DriverGetInfoRequest) (*deploymentspec.DriverGetInfoResponse, error) {
	if id.provisioner == "" {
		klog.ErrorS(errors.New("provisioner name cannot be empty"), "Invalid argument")
		return nil, status.Error(codes.InvalidArgument, "ProvisionerName is empty")
	}

	return &deploymentspec.DriverGetInfoResponse{
		Name: id.provisioner,
	}, nil
}

func MakeDeploymentID() string {
	alpha := "abcdefghijklmnopqrstuvwxyz"
	r := rand.Intn(len(alpha))
	return fmt.Sprintf("%c%s", alpha[r], rand.String(9))
}
