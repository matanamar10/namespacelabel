package labelmanager

import (
	"os"
	"strings"

	"github.com/go-logr/logr"
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	"github.com/matanamar10/namesapcelabel/internal/pkg/set"
)

// ApplyNamespaceLabel applies labels from a NamespaceLabel to the given labels map.
func ApplyNamespaceLabel(nsLabel danateamv1.NamespaceLabel, labels map[string]string, protectedLabels set.Set[string], logger logr.Logger) {
	for key, value := range nsLabel.Spec.Labels {
		if !protectedLabels.Contains(key) {
			labels[key] = value
			logger.Info("Applying labelmanager", "key", key, "value", value, "namespaceLabel", nsLabel.Name)
		} else {
			logger.Info("Skipping protected labelmanager", "labelmanager", key)
		}
	}
}

// LoadProtectedLabelsFromEnv loads protected labels from environment variables.
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
