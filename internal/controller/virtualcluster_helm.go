package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/helm"
)

func (in *VirtualClusterController) deployVCluster(ctx context.Context, vCluster *v1alpha1.VirtualCluster) error {
	deployer, err := helm.New(
		helm.WithReleaseName(vCluster.Name),
		helm.WithReleaseNamespace(vCluster.Namespace),
		helm.WithRepository(vCluster.Spec.Helm.GetVCluster().GetRepoUrl()),
		helm.WithChartName(vCluster.Spec.Helm.GetVCluster().GetChartName()),
		// TODO: add values
	)
	if err != nil {
		return err
	}

	return deployer.Upgrade(true)
}

func (in *VirtualClusterController) deployAgent(ctx context.Context, vCluster *v1alpha1.VirtualCluster) error {
	kubeconfig, err := in.handleKubeconfigRef(ctx, vCluster)
	if err != nil {
		return err
	}

	deployer, err := helm.New(
		helm.WithReleaseName(v1alpha1.AgentDefaultReleaseName),
		helm.WithReleaseNamespace(v1alpha1.AgentDefaultNamespace),
		helm.WithRepository(vCluster.Spec.Helm.GetAgent().GetRepoUrl()),
		helm.WithChartName(vCluster.Spec.Helm.GetAgent().GetChartName()),
		helm.WithKubeconfig(kubeconfig),
		// TODO: add values
	)
	if err != nil {
		return err
	}

	return deployer.Upgrade(true)
}

func (in *VirtualClusterController) handleKubeconfigRef(ctx context.Context, vCluster *v1alpha1.VirtualCluster) (string, error) {
	secret := &corev1.Secret{}

	if err := in.Get(
		ctx,
		client.ObjectKey{Name: vCluster.Spec.KubeconfigRef.Name, Namespace: vCluster.Namespace},
		secret,
	); err != nil {
		return "", err
	}

	kubeconfig, exists := secret.Data[v1alpha1.VClusterKubeconfigSecretKey]
	if !exists {
		return "", fmt.Errorf("secret %s/%s does not contain kubeconfig", vCluster.Namespace, vCluster.Spec.KubeconfigRef.Name)
	}

	return string(kubeconfig), nil
}
