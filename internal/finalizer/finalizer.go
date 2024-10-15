package finalizer

import (
	"context"

	"github.com/go-logr/logr"
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	"github.com/matanamar10/namesapcelabel/internal/cleanup"
	"github.com/matanamar10/namesapcelabel/internal/pkg/client"
	"k8s.io/client-go/tools/record"
)

const finalizerName = "namespacelabel.finalizers.namespace"

func AddFinalizer(ctx context.Context, client client.NamespaceLabelClient, logger logr.Logger, nsLabel *danateamv1.NamespaceLabel) error {
	if !containsString(nsLabel.GetFinalizers(), finalizerName) {
		nsLabel.SetFinalizers(append(nsLabel.GetFinalizers(), finalizerName))
		if err := client.UpdateNamespaceLabelStatus(ctx, nsLabel); err != nil {
			return err
		}
		logger.Info("Added finalizer to NamespaceLabel", "NamespaceLabel", nsLabel.Name)
	}
	return nil
}

func HandleFinalizer(ctx context.Context, client client.NamespaceLabelClient, logger logr.Logger, recorder record.EventRecorder, nsLabel *danateamv1.NamespaceLabel) error {
	if containsString(nsLabel.GetFinalizers(), finalizerName) {
		if err := cleanup.PerformCleanup(ctx, client, nsLabel); err != nil {
			return err
		}

		nsLabel.SetFinalizers(removeString(nsLabel.GetFinalizers(), finalizerName))
		if err := client.UpdateNamespaceLabelStatus(ctx, nsLabel); err != nil {
			return err
		}

		logger.Info("Removed finalizer and cleaned up NamespaceLabel", "NamespaceLabel", nsLabel.Name)
		recorder.Eventf(nsLabel, "Normal", "FinalizerRemoved", "Finalizer removed for NamespaceLabel %s", nsLabel.Name)
	}
	return nil
}

func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func removeString(slice []string, str string) []string {
	var result []string
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}
	return result
}
