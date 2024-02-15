package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/util"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"

	"k8s.io/klog/v2"
	"sigs.k8s.io/cli-utils/pkg/flowcontrol"
)

func AsName(val string) string {
	return strings.ReplaceAll(val, " ", "-")
}

func MarkCondition(set func(condition metav1.Condition), conditionType v1alpha1.ConditionType, conditionStatus metav1.ConditionStatus, conditionReason v1alpha1.ConditionReason, message string, messageArgs ...interface{}) {
	set(metav1.Condition{
		Type:    conditionType.String(),
		Status:  conditionStatus,
		Reason:  conditionReason.String(),
		Message: fmt.Sprintf(message, messageArgs...),
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
		Insecure:   from.TLSClientConfig.Insecure,
		ServerName: from.TLSClientConfig.ServerName,
		CertFile:   from.TLSClientConfig.CertFile,
		KeyFile:    from.TLSClientConfig.KeyFile,
		CAFile:     from.TLSClientConfig.CAFile,
		CertData:   from.TLSClientConfig.CertData,
		KeyData:    from.TLSClientConfig.KeyData,
		CAData:     from.TLSClientConfig.CAData,
		NextProtos: from.TLSClientConfig.NextProtos,
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
		klog.Info("flow control enabled, disabling client side throttling")
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

func CheckNamespace(clientset kubernetes.Clientset, namespace string) error {
	if namespace == "" {
		return nil
	}
	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}
