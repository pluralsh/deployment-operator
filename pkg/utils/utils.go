package utils

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UnstructuredToConditions(c []interface{}) []metav1.Condition {
	conditions := make([]metav1.Condition, len(c))
	for i, c := range c {
		m := c.(map[string]interface{})
		conditions[i] = metav1.Condition{
			Type:   m["type"].(string),
			Status: metav1.ConditionStatus(m["status"].(string)),
			// LastTransitionTime is required, so if it doesn't exist, set it to now
			LastTransitionTime: func() metav1.Time {
				if t, ok := m["lastTransitionTime"].(string); ok {
					parsedTime, err := time.Parse(time.RFC3339, t)
					if err == nil {
						return metav1.Time{Time: parsedTime}
					}
				}
				return metav1.Time{Time: time.Now()}
			}(),
		}
	}
	return conditions
}
