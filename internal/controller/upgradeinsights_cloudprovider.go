package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
)

type CloudProvider interface {
	UpgradeInsights(context.Context, v1alpha1.UpgradeInsights) ([]console.UpgradeInsightAttributes, error)
}

type EKSCloudProvider struct {
	kubeClient  runtimeclient.Client
	clusterName string
}

func (in *EKSCloudProvider) UpgradeInsights(ctx context.Context, ui v1alpha1.UpgradeInsights) ([]console.UpgradeInsightAttributes, error) {
	client, err := in.client(ctx, ui)
	if err != nil {
		return nil, err
	}

	insights, err := in.listInsights(ctx, client, ui)
	if err != nil {
		return nil, err
	}

	return algorithms.Map(insights, func(insight types.InsightSummary) console.UpgradeInsightAttributes {
		var refreshedAt *string
		if insight.LastRefreshTime != nil {
			refreshedAt = lo.ToPtr(insight.LastRefreshTime.Format(time.RFC3339))
		}

		var transitionedAt *string
		if insight.LastTransitionTime != nil {
			transitionedAt = lo.ToPtr(insight.LastTransitionTime.Format(time.RFC3339))
		}

		return console.UpgradeInsightAttributes{
			Name:           lo.FromPtr(insight.Name),
			Version:        insight.KubernetesVersion,
			Description:    insight.Description,
			Status:         in.fromInsightStatus(insight.InsightStatus),
			RefreshedAt:    refreshedAt,
			TransitionedAt: transitionedAt,
		}
	}), nil
}

func (in *EKSCloudProvider) listInsights(ctx context.Context, client *eks.Client, ui v1alpha1.UpgradeInsights) ([]types.InsightSummary, error) {
	var result []types.InsightSummary

	out, err := client.ListInsights(ctx, &eks.ListInsightsInput{
		ClusterName: lo.ToPtr(ui.Spec.GetClusterName(in.clusterName)),
	})
	if err != nil {
		return nil, err
	}

	result = out.Insights
	nextToken := out.NextToken
	for out.NextToken != nil {
		out, err = client.ListInsights(ctx, &eks.ListInsightsInput{
			ClusterName: lo.ToPtr(ui.Spec.GetClusterName(in.clusterName)),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, err
		}

		nextToken = out.NextToken
	}

	return result, nil
}

func (in *EKSCloudProvider) fromInsightStatus(status *types.InsightStatus) *console.UpgradeInsightStatus {
	if status == nil {
		return nil
	}

	switch status.Status {
	case types.InsightStatusValuePassing:
		return lo.ToPtr(console.UpgradeInsightStatusPassing)
	case types.InsightStatusValueError:
	case types.InsightStatusValueWarning:
		return lo.ToPtr(console.UpgradeInsightStatusFailed)
	case types.InsightStatusValueUnknown:
		return lo.ToPtr(console.UpgradeInsightStatusUnknown)
	}

	return nil
}

func (in *EKSCloudProvider) config(ctx context.Context, ui v1alpha1.UpgradeInsights) (aws.Config, error) {
	// If credentials are not provided in the request, then use default credentials.
	if ui.Spec.Credentials == nil || ui.Spec.Credentials.AWS == nil {
		return awsconfig.LoadDefaultConfig(ctx)
	}

	// Otherwise use provided credentials.
	credentials := ui.Spec.Credentials.AWS
	secretAccessKey, err := in.handleSecretAccessKeyRef(ctx, ui.Spec.Credentials.AWS.SecretAccessKeyRef, ui.Namespace)
	if err != nil {
		return aws.Config{}, err
	}

	config, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}

	config.Region = credentials.Region
	config.Credentials = awscredentials.NewStaticCredentialsProvider(
		credentials.AccessKeyID, secretAccessKey, "")

	return config, nil
}

func (in *EKSCloudProvider) handleSecretAccessKeyRef(ctx context.Context, ref corev1.SecretReference, namespace string) (string, error) {
	secret := &corev1.Secret{}

	if err := in.kubeClient.Get(
		ctx,
		runtimeclient.ObjectKey{Name: ref.Name, Namespace: ref.Namespace},
		secret,
	); err != nil {
		return "", err
	}

	key := "secretAccessKey"
	value, exists := secret.Data[key]
	if !exists {
		return "", fmt.Errorf("secret %s/%s does not contain key %s", namespace, ref.Name, key)
	}

	return string(value), nil
}

func (in *EKSCloudProvider) client(ctx context.Context, ui v1alpha1.UpgradeInsights) (*eks.Client, error) {
	config, err := in.config(ctx, ui)
	if err != nil {
		return nil, err
	}

	return eks.NewFromConfig(config), nil
}

func newEKSCloudProvider(kubeClient runtimeclient.Client, clusterName string) CloudProvider {
	return &EKSCloudProvider{
		kubeClient:  kubeClient,
		clusterName: clusterName,
	}
}

func NewCloudProvider(distro *console.ClusterDistro, kubeClient runtimeclient.Client, clusterName string) (CloudProvider, error) {
	if distro == nil {
		return nil, fmt.Errorf("distro cannot be nil")
	}

	switch *distro {
	case console.ClusterDistroEks:
		return newEKSCloudProvider(kubeClient, clusterName), nil
	}

	return nil, fmt.Errorf("unsupported distro: %s", *distro)
}
