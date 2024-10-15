package helpers

import (
	"os"
	"strings"
	"time"

	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	"github.com/matanamar10/namesapcelabel/internal/pkg/set"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func LoadProtectedLabelsFromEnv() set.Set[string] {
	labels := os.Getenv("PROTECTED_LABELS")
	if labels == "" {
		labels = "kubernetes.io/managed-by,kubernetes.io/created-by,control-plane,cluster-owner"
	}

	protectedSet := set.NewSet[string]()
	for _, label := range strings.Split(labels, ",") {
		protectedSet.Add(label)
	}
	return protectedSet
}

// UpdateCondition updates the conditions in the NamespaceLabel status.
func UpdateCondition(status *danateamv1.NamespaceLabelStatus, conditionType, statusValue, reason, message string) {
	now := metav1.NewTime(time.Now())
	for i, condition := range status.Conditions {
		if condition.Type == conditionType {
			status.Conditions[i].Status = statusValue
			status.Conditions[i].Reason = reason
			status.Conditions[i].Message = message
			status.Conditions[i].LastTransitionTime = now
			return
		}
	}

	status.Conditions = append(status.Conditions, danateamv1.Condition{
		Type:               conditionType,
		Status:             statusValue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}
