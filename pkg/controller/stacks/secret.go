package stacks

import (
	"context"

	"github.com/pluralsh/deployment-operator/cmd/harness/args"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	jobRunSecretName = "job-run-env"
)

func (r *StackReconciler) getRunSecretData() map[string]string {
	return map[string]string{
		args.EnvConsoleUrl:   r.consoleURL,
		args.EnvConsoleToken: r.deployToken,
	}
}

func (r *StackReconciler) hasRunSecretData(data map[string][]byte) bool {
	token, hasToken := data[args.EnvConsoleToken]
	url, hasUrl := data[args.EnvConsoleUrl]
	return hasToken && hasUrl && string(token) == r.deployToken && string(url) == r.consoleURL
}

func (r *StackReconciler) upsertRunSecret(ctx context.Context) (*corev1.Secret, error) {
	logger := log.FromContext(ctx)
	secret := &corev1.Secret{}

	if err := r.k8sClient.Get(ctx, types.NamespacedName{Name: jobRunSecretName, Namespace: r.namespace}, secret); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		logger.V(2).Info("generating secret", "namespace", r.namespace, "name", jobRunSecretName)
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: jobRunSecretName, Namespace: r.namespace},
			StringData: r.getRunSecretData(),
		}

		logger.V(2).Info("creating secret", "namespace", secret.Namespace, "name", secret.Name)
		if err := r.k8sClient.Create(ctx, secret); err != nil {
			logger.Error(err, "unable to create secret")
			return nil, err
		}

		return secret, nil
	}

	if r.hasRunSecretData(secret.Data) {
		logger.V(2).Info("updating secret", "namespace", secret.Namespace, "name", secret.Name)
		secret.StringData = r.getRunSecretData()
		if err := r.k8sClient.Update(ctx, secret); err != nil {
			logger.Error(err, "unable to update secret")
			return nil, err
		}
	}

	return secret, nil

}