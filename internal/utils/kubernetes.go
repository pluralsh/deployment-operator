package utils

import (
	"fmt"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MarkCondition(set func(condition metav1.Condition), conditionType v1alpha1.ConditionType, conditionStatus metav1.ConditionStatus, conditionReason v1alpha1.ConditionReason, message string, messageArgs ...interface{}) {
	set(metav1.Condition{
		Type:    conditionType.String(),
		Status:  conditionStatus,
		Reason:  conditionReason.String(),
		Message: fmt.Sprintf(message, messageArgs...),
	})
}
