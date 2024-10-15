package status

import (
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// UpdateCondition updates or appends a condition in the NamespaceLabelStatus conditions list.
func UpdateCondition(status *danateamv1.NamespaceLabelStatus, conditionType, statusValue, reason, message string) {
	// Fetch current time once to avoid calling time.Now() repeatedly
	now := metav1.NewTime(time.Now())

	// Iterate over the conditions and update if found
	for i, condition := range status.Conditions {
		if condition.Type == conditionType {
			// Update the condition if it matches the type
			updateConditionFields(&status.Conditions[i], statusValue, reason, message, now)
			return // Exit early since the condition has been updated
		}
	}

	// Append new condition if the conditionType doesn't exist
	status.Conditions = append(status.Conditions, danateamv1.Condition{
		Type:               conditionType,
		Status:             statusValue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

// updateConditionFields is a helper function to update the fields of a Condition object.
func updateConditionFields(condition *danateamv1.Condition, statusValue, reason, message string, now metav1.Time) {
	condition.Status = statusValue
	condition.Reason = reason
	condition.Message = message
	condition.LastTransitionTime = now
}
