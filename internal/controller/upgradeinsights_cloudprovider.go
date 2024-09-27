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
	"sigs.k8s.io/controller-runtime/pkg/log"

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

	return algorithms.Map(insights, func(insight *types.Insight) console.UpgradeInsightAttributes {
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
			Details:        in.toInsightDetails(insight),
			RefreshedAt:    refreshedAt,
			TransitionedAt: transitionedAt,
		}
	}), nil
}

func (in *EKSCloudProvider) listInsights(ctx context.Context, client *eks.Client, ui v1alpha1.UpgradeInsights) ([]*types.Insight, error) {
	logger := log.FromContext(ctx)
	var result []types.InsightSummary

	out, err := client.ListInsights(ctx, &eks.ListInsightsInput{
		ClusterName: lo.ToPtr(in.clusterName),
	})
	if err != nil {
		return nil, err
	}

	result = out.Insights
	nextToken := out.NextToken
	for out.NextToken != nil {
		out, err = client.ListInsights(ctx, &eks.ListInsightsInput{
			ClusterName: lo.ToPtr(in.clusterName),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, err
		}

		nextToken = out.NextToken
	}

	return algorithms.Filter(
		algorithms.Map(result, func(insight types.InsightSummary) *types.Insight {
			output, err := client.DescribeInsight(ctx, &eks.DescribeInsightInput{
				ClusterName: lo.ToPtr(in.clusterName),
				Id:          insight.Id,
			})
			// If there is an error getting the details of an insight just ignore.
			// It will be picked up during the next reconcile.
			if err != nil {
				logger.Error(err, "could not describe insight", "clusterName", in.clusterName, "id", insight.Id)
				return nil
			}

			return output.Insight
		}), func(insight *types.Insight) bool {
			return insight != nil
		}), nil
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

func (in *EKSCloudProvider) fromClientStats(stats []types.ClientStat) *console.UpgradeInsightStatus {
	const failedBeforeDuration = int64(24 * time.Hour)
	for _, stat := range stats {
		if stat.LastRequestTime != nil && time.Now().Sub(*stat.LastRequestTime).Milliseconds() < failedBeforeDuration {
			return lo.ToPtr(console.UpgradeInsightStatusFailed)
		}
	}

	return lo.ToPtr(console.UpgradeInsightStatusPassing)
}

func (in *EKSCloudProvider) toInsightDetails(insight *types.Insight) []*console.UpgradeInsightDetailAttributes {
	if insight.CategorySpecificSummary == nil {
		return nil
	}

	result := make([]*console.UpgradeInsightDetailAttributes, 0)
	for _, r := range insight.CategorySpecificSummary.DeprecationDetails {
		result = append(result, &console.UpgradeInsightDetailAttributes{
			Used:        r.Usage,
			Replacement: r.ReplacedWith,
			ReplacedIn:  r.StartServingReplacementVersion,
			RemovedIn:   r.StopServingVersion,
			Status:      in.fromClientStats(r.ClientStats),
		})
	}

	return result
}

func (in *EKSCloudProvider) config(ctx context.Context, ui v1alpha1.UpgradeInsights) (aws.Config, error) {
	// If credentials are not provided in the request, then use default credentials.
	if ui.Spec.Credentials == nil || ui.Spec.Credentials.AWS == nil {
		return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithEC2IMDSRegion())
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

	if *distro == console.ClusterDistroEks {
		return newEKSCloudProvider(kubeClient, clusterName), nil
	}

	return nil, fmt.Errorf("unsupported distro: %s", *distro)
}
