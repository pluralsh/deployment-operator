package client

import (
	"fmt"

	console "github.com/pluralsh/console-client-go"
	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/apis/pipelines/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func (c *Client) UpdateGate(id string, attributes console.GateUpdateAttributes) error {
	_, err := c.consoleClient.UpdateGate(c.ctx, id, attributes)
	return err
}

func (c *Client) GetClusterGates() ([]*console.PipelineGateFragment, error) {
	resp, err := c.consoleClient.GetClusterGates(c.ctx)
	if err != nil {
		return nil, err
	}

	return resp.ClusterGates, nil
}

//type PipelineGateFragment struct {
//	ID        string            "json:\"id\" graphql:\"id\""
//	Name      string            "json:\"name\" graphql:\"name\""
//	Type      GateType          "json:\"type\" graphql:\"type\""
//	State     GateState         "json:\"state\" graphql:\"state\""
//	UpdatedAt *string           "json:\"updatedAt\" graphql:\"updatedAt\""
//	Spec      *GateSpecFragment "json:\"spec\" graphql:\"spec\""
//}
//
//type GateType string
//
//const (
//	GateTypeApproval GateType = "APPROVAL"
//	GateTypeWindow   GateType = "WINDOW"
//	GateTypeJob      GateType = "JOB"
//)
//
//type GateState string
//
//const (
//	GateStatePending GateState = "PENDING"
//	GateStateOpen    GateState = "OPEN"
//	GateStateClosed  GateState = "CLOSED"
//)
//
//type GateSpecFragment struct {
//	Job *struct {
//		Namespace  string  "json:\"namespace\" graphql:\"namespace\""
//		Raw        *string "json:\"raw\" graphql:\"raw\""
//		Containers []*struct {
//			Image string    "json:\"image\" graphql:\"image\""
//			Args  []*string "json:\"args\" graphql:\"args\""
//			Env   []*struct {
//				Name  string "json:\"name\" graphql:\"name\""
//				Value string "json:\"value\" graphql:\"value\""
//			} "json:\"env\" graphql:\"env\""
//			EnvFrom []*struct {
//				ConfigMap string "json:\"configMap\" graphql:\"configMap\""
//				Secret    string "json:\"secret\" graphql:\"secret\""
//			} "json:\"envFrom\" graphql:\"envFrom\""
//		} "json:\"containers\" graphql:\"containers\""
//		Labels         map[string]interface{} "json:\"labels\" graphql:\"labels\""
//		Annotations    map[string]interface{} "json:\"annotations\" graphql:\"annotations\""
//		ServiceAccount *string                "json:\"serviceAccount\" graphql:\"serviceAccount\""
//	} "json:\"job\" graphql:\"job\""
//}
//
//type PipelineGate struct {
//	metav1.TypeMeta   `json:",inline"`            // name and apiVersion
//	metav1.ObjectMeta `json:"metadata,omitempty"` // namespace, labels, annotations
//
//	Spec   PipelineGateSpec   `json:"spec,omitempty"`
//	Status PipelineGateStatus `json:"status,omitempty"`
//}
//
//// PipelineGateStatus defines the observed state of ConfigurationOverlay
//type PipelineGateStatus struct {
//	State GateState `json:"state"`
//}
//
//// PipelineGateSpec defines the detailed gate specifications
//type PipelineGateSpec struct {
//	ID       string   `json:"id"`
//	Name     string   `json:"name"`
//	Type     GateType `json:"type"`
//	GateSpec GateSpec `json:"gateSpec"`
//}
//
//// GateSpec defines the detailed gate specifications
//type GateSpec struct {
//	JobSpec batchv1.JobSpec `json:"jobSpec"`
//}

func (c *Client) ParsePipelineGateCR(fragment *console.PipelineGateFragment) (*pipelinesv1alpha1.PipelineGate, error) {
	// Create a PipelineGate instance
	pipelineGate := &pipelinesv1alpha1.PipelineGate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PipelineGate",
			APIVersion: "yourgroup/version",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fragment.Name,
			Namespace: fragment.Spec.Job.Namespace,
			// You may set other metadata fields as needed (namespace, labels, annotations)
		},
		Spec: pipelinesv1alpha1.PipelineGateSpec{
			ID:       fragment.ID,
			Name:     fragment.Name,
			Type:     pipelinesv1alpha1.GateType(fragment.Type),
			GateSpec: pipelinesv1alpha1.GateSpec{convertToJobSpec(&fragment.Spec.Job)},
		},
		Status: pipelinesv1alpha1.PipelineGateStatus{
			State: pipelinesv1alpha1.GateState(fragment.State),
		},
	}

	return pipelineGate, nil
}

func parseYAMLIntoJob(yamlString string) (*batchv1.Job, error) {
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

	return nil, fmt.Errorf("parsed object is not of type JobSpec")
}

// //////////////// Helper function to convert the GateSpecFragment.Job into batchv1.JobSpec
func convertToJobSpec(fragment *console.JobSpecFragment) (batchv1.JobSpec, error) {
	//type JobSpecFragment struct {
	//	Namespace      string                   "json:\"namespace\" graphql:\"namespace\""
	//	Raw            *string                  "json:\"raw\" graphql:\"raw\""
	//	Containers     []*ContainerSpecFragment "json:\"containers\" graphql:\"containers\""
	//	Labels         map[string]interface{}   "json:\"labels\" graphql:\"labels\""
	//	Annotations    map[string]interface{}   "json:\"annotations\" graphql:\"annotations\""
	//	ServiceAccount *string                  "json:\"serviceAccount\" graphql:\"serviceAccount\""
	//}
	if fragment == nil {
		return batchv1.JobSpec{}
	}

	if fragment.Raw != nil {
		job := parseYAMLIntoJob(*fragment.Raw)
		return parseYAMLIntoJob(*fragment.Raw)
	}

	// Create a basic JobSpec
	//jobSpec := batchv1.JobSpec{
	//	metav1.ObjectMeta{
	//		Name:      fragment.Name,
	//		Namespace: fragment.Namespace,
	//	},
	//	Template: corev1.PodTemplateSpec{
	//		Spec: corev1.PodSpec{
	//			Containers: convertToContainers(fragment.Containers),
	//			Labels:     fragment.Labels,
	//			// Set other fields as needed
	//		},
	//	},
	//	// Set other fields as needed
	//}

	return jobSpec
}

//
//// Helper function to convert []*console.GateSpecFragmentJobContainer into []v1.Container
//func convertToContainers(*console.GateSpecFragment) []v1.Container {
//	var containers []v1.Container
//
//	for _, containerFragment := range containerFragments {
//		container := v1.Container{
//			Name:  containerFragment.Image,
//			Image: containerFragment.Image,
//			Args:  containerFragment.Args,
//			Env:   convertToEnvVars(containerFragment.Env),
//			// Set other fields as needed
//		}
//
//		containers = append(containers, container)
//	}
//
//	return containers
//}
//
//// Helper function to convert []*console.GateSpecFragmentJobContainerEnv into []v1.EnvVar
//func convertToEnvVars(envFragments []*console.GateSpecFragment) []v1.EnvVar {
//	var envVars []corev1.EnvVar
//
//	for _, envFragment := range envFragments {
//		envVar := corev1.EnvVar{
//			Name:  envFragment.Name,
//			Value: envFragment.Value,
//		}
//
//		envVars = append(envVars, envVar)
//	}
//
//	return envVars
//}
