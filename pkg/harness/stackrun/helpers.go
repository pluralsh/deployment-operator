package stackrun

import (
	"context"
	"time"

	gqlclient "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	console "github.com/pluralsh/deployment-operator/pkg/client"
)

func MarkStackRun(client console.Client, id string, status gqlclient.StackStatus) error {
	return client.UpdateStackRun(id, gqlclient.StackRunAttributes{
		Status: status,
	})
}

func MarkStackRunWithRetry(client console.Client, id string, status gqlclient.StackStatus, interval time.Duration) {
	// Ignore error since we never return it from the condition function.
	_ = wait.PollUntilContextCancel(context.Background(), interval, true, func(ctx context.Context) (done bool, err error) {
		err = MarkStackRun(client, id, status)
		if err != nil {
			klog.Errorf("stack run update failed: %v", err)
			return false, nil
		}

		return true, nil
	})
}

func StartStackRun(client console.Client, id string) error {
	return MarkStackRun(client, id, gqlclient.StackStatusRunning)
}

func CompleteStackRun(client console.Client, id string) error {
	return MarkStackRun(client, id, gqlclient.StackStatusSuccessful)
}

func CancelStackRun(client console.Client, id string) error {
	return MarkStackRun(client, id, gqlclient.StackStatusCancelled)
}

func FailStackRun(client console.Client, id string) error {
	return MarkStackRun(client, id, gqlclient.StackStatusFailed)
}

func MarkStackRunStep(client console.Client, id string, status gqlclient.StepStatus) error {
	return client.UpdateStackRunStep(id, gqlclient.RunStepAttributes{
		Status: status,
	})
}

func StartStackRunStep(client console.Client, id string) error {
	return MarkStackRunStep(client, id, gqlclient.StepStatusRunning)
}

func CompleteStackRunStep(client console.Client, id string) error {
	return MarkStackRunStep(client, id, gqlclient.StepStatusSuccessful)
}

func FailStackRunStep(client console.Client, id string) error {
	return MarkStackRunStep(client, id, gqlclient.StepStatusFailed)
}
