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
	envConsoleUrl   = "PLRL_CONSOLE_URL"
	envConsoleToken = "PLRL_CONSOLE_TOKEN"
)

func (r *StackReconciler) getRunSecretData() map[string]string {
	return map[string]string{
		envConsoleUrl:   r.consoleURL,
		envConsoleToken: r.deployToken,
	}
}

func (r *StackReconciler) hasRunSecretData(data map[string][]byte) bool {
	token, hasToken := data[envConsoleToken]
	url, hasUrl := data[envConsoleUrl]
	return hasToken && hasUrl && string(token) == r.deployToken && string(url) == r.consoleURL
}

func (r *StackReconciler) upsertRunSecret(ctx context.Context, name, namespace string) (*corev1.Secret, error) {
	logger := log.FromContext(ctx)
	secret := &corev1.Secret{}

	if err := r.k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, secret); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		logger.V(2).Info("generating run job secret", "namespace", namespace, "name", name)
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			StringData: r.getRunSecretData(),
		}

		logger.V(2).Info("creating secret", "namespace", secret.Namespace, "name", secret.Name)
		if err := r.k8sClient.Create(ctx, secret); err != nil {
			logger.Error(err, "unable to create secret")
			return nil, err
		}

		return secret, nil
	}

	if !r.hasRunSecretData(secret.Data) {
		logger.V(2).Info("updating secret", "namespace", secret.Namespace, "name", secret.Name)
		secret.StringData = r.getRunSecretData()
		if err := r.k8sClient.Update(ctx, secret); err != nil {
			logger.Error(err, "unable to update secret")
			return nil, err
		}
	}

	return secret, nil
}
