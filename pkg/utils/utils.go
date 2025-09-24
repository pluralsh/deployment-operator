package utils

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UnstructuredToConditions(c []interface{}) []metav1.Condition {
	conditions := make([]metav1.Condition, 0)
	for _, c := range c {
		m := c.(map[string]interface{})
		t, tOk := m["type"].(string)
		s, sOk := m["status"].(string)
		tt, _ := m["lastTransitionTime"].(string)
		if tOk && sOk {
			conditions = append(conditions, metav1.Condition{
				Type:   t,
				Status: metav1.ConditionStatus(s),
				LastTransitionTime: func() metav1.Time {
					parsedTime, err := time.Parse(time.RFC3339, tt)
					if err == nil {
						return metav1.Time{Time: parsedTime}
					}

					return metav1.Time{Time: time.Now()}
				}(),
			})
		}
	}
	return conditions
}
