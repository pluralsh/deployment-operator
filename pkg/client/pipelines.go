package client

import (
	"fmt"

	console "github.com/pluralsh/console-client-go"
	v1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/internal/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func (c *client) UpdateGate(id string, attributes console.GateUpdateAttributes) error {
	_, err := c.consoleClient.UpdateGate(c.ctx, id, attributes)
	return err
}

func (c *client) GetClusterGates() ([]*console.PipelineGateFragment, error) {
	resp, err := c.consoleClient.GetClusterGates(c.ctx)
	if err != nil {
		return nil, err
	}

	return resp.ClusterGates, nil
}

func (c *client) GetClusterGate(id string) (*console.PipelineGateFragment, error) {
	resp, err := c.consoleClient.GetClusterGate(c.ctx, id)
	if err != nil {
		return nil, fmt.Errorf("gate with id %s not found", id)

	}
	return resp.ClusterGate, nil
}

func (c *client) GateExists(id string) bool {
	pgf, err := c.GetClusterGate(id)
	if pgf != nil {
		return true
	}
	if errors.IsNotFound(err) {
		return false
	}
	return err == nil
}

func (c *client) ParsePipelineGateCR(pgFragment *console.PipelineGateFragment) (*v1alpha1.PipelineGate, error) {

	now := metav1.Now()
	pipelineGate := &v1alpha1.PipelineGate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PipelineGate",
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.AsName(pgFragment.Name), // + "-" + pgFragment.ID,
			Namespace: pgFragment.Spec.Job.Namespace,
		},
		Spec: v1alpha1.PipelineGateSpec{
			ID:       pgFragment.ID,
			Name:     pgFragment.Name, // + "-" + pgFragment.ID,
			Type:     v1alpha1.GateType(pgFragment.Type),
			GateSpec: gateSpecFromGateSpecFragment(pgFragment.Name, pgFragment.Spec),
		},
	}
	gateState := v1alpha1.GateState(pgFragment.State)
	pipelineGate.Status.SyncedState = gateState
	pipelineGate.Status.LastSyncedAt = now
	return pipelineGate, nil
}

func jobFromYaml(yamlString string) (*batchv1.Job, error) {
	job := &batchv1.Job{}

	// unmarshal the YAML string into the Job rep
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(yamlString), nil, job)
	if err != nil {
		return nil, err
	}

	// ensure decoded object is actually of type Job after using universal deserializer
	if obj, ok := obj.(*batchv1.Job); ok {
		return obj, nil
	}

	return nil, fmt.Errorf("parsed object is not of type Job")
}

func gateSpecFromGateSpecFragment(gateName string, gsFragment *console.GateSpecFragment) *v1alpha1.GateSpec {
	if gsFragment == nil {
		return nil
	}
	return &v1alpha1.GateSpec{
		JobSpec: jobSpecFromJobSpecFragment(gateName, gsFragment.Job),
	}
}

func jobSpecFromJobSpecFragment(gateName string, jsFragment *console.JobSpecFragment) *batchv1.JobSpec {
	if jsFragment == nil {
		return nil
	}
	if jsFragment.Raw != nil && *jsFragment.Raw != "null" {
		job, err := jobFromYaml(*jsFragment.Raw)
		if err != nil {
			return nil
		}
		return &job.Spec
	}
	name := utils.AsName(gateName)
	jobSpec := &batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: jsFragment.Namespace,
				// convert map[string]interface{} to map[string]string
				Labels:      stringMapFromInterfaceMap(jsFragment.Labels),
				Annotations: stringMapFromInterfaceMap(jsFragment.Annotations),
			},
			Spec: corev1.PodSpec{
				Containers:    containersFromContainerSpecFragments(name, jsFragment.Containers),
				RestartPolicy: corev1.RestartPolicyOnFailure,
			},
		},
	}
	// Add the gatename annotation
	if jobSpec.Template.ObjectMeta.Annotations == nil {
		jobSpec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	gateNameAnnotationKey := v1alpha1.GroupVersion.Group + "/gatename"
	jobSpec.Template.ObjectMeta.Annotations[gateNameAnnotationKey] = gateName

	return jobSpec
}

func containersFromContainerSpecFragments(gateName string, containerSpecFragments []*console.ContainerSpecFragment) []corev1.Container {
	var containers []corev1.Container

	for i, csFragment := range containerSpecFragments {
		if csFragment == nil {
			continue
		}

		container := corev1.Container{
			// todo: maybe add a name to the graphql api too? for now let's use the gate name plus the container fragment index
			Name:  fmt.Sprintf("%s-%d", utils.AsName(gateName), i),
			Image: csFragment.Image,
			Args:  make([]string, 0),
		}

		for _, arg := range csFragment.Args {
			container.Args = append(container.Args, *arg)
		}

		for _, envVar := range csFragment.Env {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  envVar.Name,
				Value: envVar.Value,
			})
		}

		// translate the EnvFrom structs from the fragment into the according corev1.EnvFromSource of the k8s pod api
		for _, envFrom := range csFragment.EnvFrom {
			container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: envFrom.ConfigMap,
					},
				},
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: envFrom.Secret,
					},
				},
			})
		}

		containers = append(containers, container)
	}

	return containers
}

func stringMapFromInterfaceMap(labels map[string]interface{}) map[string]string {
	result := make(map[string]string)

	for key, value := range labels {
		if strValue, ok := value.(string); ok {
			result[key] = strValue
		} else {
			fmt.Printf("Skipping non-string value for key %s\n", key)
		}
	}
	return result
}
