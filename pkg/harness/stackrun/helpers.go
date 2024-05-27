package stackrun

import (
	gqlclient "github.com/pluralsh/console-client-go"

	console "github.com/pluralsh/deployment-operator/pkg/client"
)

func MarkStackRun(client console.Client, id string, status gqlclient.StackStatus) error {
	return client.UpdateStackRun(id, gqlclient.StackRunAttributes{
		Status: status,
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
