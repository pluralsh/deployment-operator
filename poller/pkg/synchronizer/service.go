package synchronizer

import (
	console "github.com/pluralsh/console-client-go"
	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func toKubernetesServices(svcs []*console.ServiceDeploymentFragment) []platform.Deployment {
	var services []platform.Deployment
	for _, svc := range svcs {
		services = append(services, toKubernetesService(svc))
	}
	return services
}

// TODO: Figure it out.
func toKubernetesService(svc *console.ServiceDeploymentFragment) platform.Deployment {
	return platform.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			// Using ID as name simplifies sync logic as ID cannot be changed.
			// We will store display name in spec.
			Name: svc.ID,
			// Can namespace be changed?
			Namespace: svc.Namespace,
		},
		Spec: platform.DeploymentSpec{
			Git: platform.GitRef{
				Ref:    svc.Git.Ref,
				Folder: svc.Git.Folder,
			},
			Namespace:           "test",
			ProviderName:        "argocd.platform.plural.sh",
			DeploymentClassName: "argocd",
		},
	}
}
