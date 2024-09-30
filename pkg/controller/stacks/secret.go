package stacks

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	secretName           = "job-run-secret"
	secretDeployTokenKey = "deploy-token"
)

func (r *StackReconciler) upsertRunSecret(ctx context.Context) (*corev1.Secret, error) {
	logger := log.FromContext(ctx)
	secret := &corev1.Secret{}

	if err := r.k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: r.namespace}, secret); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		logger.V(2).Info("generating secret", "namespace", r.namespace, "name", secretName)
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: r.namespace},
			StringData: map[string]string{secretDeployTokenKey: r.deployToken},
		}

		logger.V(2).Info("creating secret", "namespace", secret.Namespace, "name", secret.Name)
		if err := r.k8sClient.Create(ctx, secret); err != nil {
			logger.Error(err, "unable to create secret")
			return nil, err
		}

		return secret, nil
	}

	if deployToken, exists := secret.Data[secretDeployTokenKey]; !exists || string(deployToken) != r.deployToken {
		logger.V(2).Info("updating secret", "namespace", secret.Namespace, "name", secret.Name)
		secret.StringData = map[string]string{secretDeployTokenKey: r.deployToken}
		if err := r.k8sClient.Update(ctx, secret); err != nil {
			logger.Error(err, "unable to update secret")
			return nil, err
		}
	}

	return secret, nil

}
