package utils

import (
	"context"
	"fmt"
	"maps"
	"os"
	"reflect"
	"strings"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/flowcontrol"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
)

func TryAddControllerRef(ctx context.Context, client ctrlruntimeclient.Client, owner ctrlruntimeclient.Object, controlled ctrlruntimeclient.Object, scheme *runtime.Scheme) error {
	key := ctrlruntimeclient.ObjectKeyFromObject(controlled)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := client.Get(ctx, key, controlled); err != nil {
			return err
		}

		if owner.GetDeletionTimestamp() != nil || controlled.GetDeletionTimestamp() != nil {
			return nil
		}

		original := controlled.DeepCopyObject().(ctrlruntimeclient.Object)

		err := controllerutil.SetControllerReference(owner, controlled, scheme)
		if err != nil {
			return err
		}

		if reflect.DeepEqual(original.GetOwnerReferences(), controlled.GetOwnerReferences()) {
			return nil
		}

		return client.Patch(ctx, controlled, ctrlruntimeclient.MergeFromWithOptions(original, ctrlruntimeclient.MergeFromWithOptimisticLock{}))
	})
}

func TryToUpdate(ctx context.Context, client ctrlruntimeclient.Client, object ctrlruntimeclient.Object) error {
	key := ctrlruntimeclient.ObjectKeyFromObject(object)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		original := object.DeepCopyObject().(ctrlruntimeclient.Object)
		if err := client.Get(ctx, key, object); err != nil {
			return fmt.Errorf("could not fetch current %s/%s state, got error: %w", object.GetName(), object.GetNamespace(), err)
		}

		if reflect.DeepEqual(object, original) {
			return nil
		}

		return client.Patch(ctx, original, ctrlruntimeclient.MergeFrom(object))
	})
}

func TryAddOwnerRef(ctx context.Context, client ctrlruntimeclient.Client, owner ctrlruntimeclient.Object, object ctrlruntimeclient.Object, scheme *runtime.Scheme) error {
	key := ctrlruntimeclient.ObjectKeyFromObject(object)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := client.Get(ctx, key, object); err != nil {
			return err
		}

		if owner.GetDeletionTimestamp() != nil || object.GetDeletionTimestamp() != nil {
			return nil
		}

		original := object.DeepCopyObject().(ctrlruntimeclient.Object)

		err := controllerutil.SetOwnerReference(owner, object, scheme)
		if err != nil {
			return err
		}

		if reflect.DeepEqual(original.GetOwnerReferences(), object.GetOwnerReferences()) {
			return nil
		}

		return client.Patch(ctx, object, ctrlruntimeclient.MergeFromWithOptions(original, ctrlruntimeclient.MergeFromWithOptimisticLock{}))
	})
}

func AsName(val string) string {
	return strings.ReplaceAll(val, " ", "-")
}

func MarkCondition(set func(condition metav1.Condition), conditionType v1alpha1.ConditionType, conditionStatus metav1.ConditionStatus, conditionReason v1alpha1.ConditionReason, message string) {
	set(metav1.Condition{
		Type:    conditionType.String(),
		Status:  conditionStatus,
		Reason:  conditionReason.String(),
		Message: message,
	})
}

func NewFactory(cfg *rest.Config) util.Factory {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.WithDiscoveryQPS(cfg.QPS).WithDiscoveryBurst(cfg.Burst)
	cfgPtrCopy := cfg
	kubeConfigFlags.WrapConfigFn = func(c *rest.Config) *rest.Config {
		// update rest.Config to pick up QPS & timeout changes
		deepCopyRESTConfig(cfgPtrCopy, c)
		return c
	}
	matchVersionKubeConfigFlags := util.NewMatchVersionFlags(kubeConfigFlags)
	return util.NewFactory(matchVersionKubeConfigFlags)
}

func NewNamespacedFactory(cfg *rest.Config, namespace string) util.Factory {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.WithDiscoveryQPS(cfg.QPS).WithDiscoveryBurst(cfg.Burst)
	kubeConfigFlags.Namespace = &namespace
	cfgPtrCopy := cfg
	kubeConfigFlags.WrapConfigFn = func(c *rest.Config) *rest.Config {
		// update rest.Config to pick up QPS & timeout changes
		deepCopyRESTConfig(cfgPtrCopy, c)
		return c
	}

	matchVersionKubeConfigFlags := util.NewMatchVersionFlags(kubeConfigFlags)
	return util.NewFactory(matchVersionKubeConfigFlags)
}

func deepCopyRESTConfig(from, to *rest.Config) {
	to.Host = from.Host
	to.APIPath = from.APIPath
	to.ContentConfig = from.ContentConfig
	to.Username = from.Username
	to.Password = from.Password
	to.BearerToken = from.BearerToken
	to.BearerTokenFile = from.BearerTokenFile
	to.Impersonate = rest.ImpersonationConfig{
		UserName: from.Impersonate.UserName,
		UID:      from.Impersonate.UID,
		Groups:   from.Impersonate.Groups,
		Extra:    from.Impersonate.Extra,
	}
	to.AuthProvider = from.AuthProvider
	to.AuthConfigPersister = from.AuthConfigPersister
	to.ExecProvider = from.ExecProvider
	if from.ExecProvider != nil && from.ExecProvider.Config != nil {
		to.ExecProvider.Config = from.ExecProvider.Config.DeepCopyObject()
	}
	to.TLSClientConfig = rest.TLSClientConfig{
		Insecure:   from.Insecure,
		ServerName: from.ServerName,
		CertFile:   from.CertFile,
		KeyFile:    from.KeyFile,
		CAFile:     from.CAFile,
		CertData:   from.CertData,
		KeyData:    from.KeyData,
		CAData:     from.CAData,
		NextProtos: from.NextProtos,
	}
	to.UserAgent = from.UserAgent
	to.DisableCompression = from.DisableCompression
	to.Transport = from.Transport
	to.WrapTransport = from.WrapTransport
	to.QPS = from.QPS
	to.Burst = from.Burst
	to.RateLimiter = from.RateLimiter
	to.WarningHandler = from.WarningHandler
	to.Timeout = from.Timeout
	to.Dial = from.Dial
	to.Proxy = from.Proxy
}

func DisableClientLimits(config *rest.Config) {
	enabled, err := flowcontrol.IsEnabled(context.Background(), config)
	if err != nil {
		klog.Error(err, "could not determine if flowcontrol was enabled")
	} else if enabled {
		klog.V(1).Info("flow control enabled, disabling client side throttling")
		config.QPS = -1
		config.Burst = -1
		config.RateLimiter = nil
	}
}

func GetOperatorNamespace() (string, error) {
	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		// get from env
		namespace := os.Getenv("OPERATOR_NAMESPACE")
		if namespace != "" {
			return namespace, nil
		}
		return "", fmt.Errorf("unable to get operator namespace: %w", err)
	}
	return string(ns), nil
}

func CheckNamespace(clientset kubernetes.Clientset, namespace string, labels, annotations map[string]string) error {
	if namespace == "" {
		return nil
	}

	ctx := context.Background()
	nsClient := clientset.CoreV1().Namespaces()
	existing, err := nsClient.Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = nsClient.Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        namespace,
					Annotations: annotations,
					Labels:      labels,
				},
			}, metav1.CreateOptions{}); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	// update labels and annotations
	if (!reflect.DeepEqual(labels, existing.Labels) && labels != nil) || (!reflect.DeepEqual(annotations, existing.Annotations) && annotations != nil) {
		maps.Copy(existing.Labels, labels)
		maps.Copy(existing.Annotations, annotations)
		if _, err := nsClient.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}
