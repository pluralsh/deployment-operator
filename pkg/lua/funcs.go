package lua

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Status struct {
	Conditions []metav1.Condition
}

func statusConditionExists(s map[string]interface{}, condition string) bool {
	sts := Status{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(s, &sts); err != nil {
		return false
	}

	return meta.FindStatusCondition(sts.Conditions, condition) != nil
}

func isStatusConditionTrue(s map[string]interface{}, condition string) bool {
	sts := Status{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(s, &sts); err != nil {
		return false
	}

	if meta.FindStatusCondition(sts.Conditions, condition) != nil {
		if meta.IsStatusConditionTrue(sts.Conditions, condition) {
			return true
		}
	}

	return false
}
