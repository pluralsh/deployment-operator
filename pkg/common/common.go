package common

import (
	"context"
	"time"

	"github.com/pluralsh/deployment-operator/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	ManagedByLabel             = "plural.sh/managed-by"
	AgentLabelValue            = "agent"
	LastProgressTimeAnnotation = "plural.sh/last-progress-time"
)

func ManagedByAgentLabelSelector() labels.Selector {
	return labels.SelectorFromSet(map[string]string{ManagedByLabel: AgentLabelValue})
}

func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	unstructured := &unstructured.Unstructured{Object: objMap}
	return unstructured, nil
}

func Unmarshal(s string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}

	return result, nil
}

func GetLastProgressTimestamp(ctx context.Context, k8sClient ctrclient.Client, obj *unstructured.Unstructured) (progressTime metav1.Time, err error) {
	progressTime = metav1.Now()

	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}
	annotations := obj.GetAnnotations()
	timeStr, ok := annotations[LastProgressTimeAnnotation]

	defer func() {
		if !ok {
			err = utils.TryToUpdate(ctx, k8sClient, obj)
			if err != nil {
				return
			}
			key := ctrclient.ObjectKeyFromObject(obj)
			err = k8sClient.Get(ctx, key, obj)
		}
	}()

	if !ok {
		annotations[LastProgressTimeAnnotation] = progressTime.Format(time.RFC3339)
		obj.SetAnnotations(annotations)
		return
	}
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return
	}
	progressTime = metav1.Time{Time: parsedTime}

	return
}
