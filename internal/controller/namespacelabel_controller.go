package controller

import (
	"context"

	"github.com/go-logr/logr"
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	"github.com/matanamar10/namesapcelabel/internal/finalizer"
	"github.com/matanamar10/namesapcelabel/internal/pkg/client"
	"github.com/matanamar10/namesapcelabel/internal/pkg/labelmanager"
	"github.com/matanamar10/namesapcelabel/internal/pkg/set"
	"github.com/matanamar10/namesapcelabel/internal/pkg/status"
	"github.com/matanamar10/namesapcelabel/internal/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

type NamespaceLabelReconciler struct {
	Client          client.NamespaceLabelClient
	Logger          logr.Logger
	ProtectedLabels set.Set[string]
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
}

// Reconcile performs the reconciliation for NamespaceLabel resources.
func (r *NamespaceLabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsLabels, err := r.Client.ListNamespaceLabels(ctx, req.Namespace)
	if err != nil {
		r.Recorder.Eventf(nil, corev1.EventTypeWarning, "NamespaceLabelListFailed", "Failed to list NamespaceLabels for namespace %s: %v", req.Namespace, err)
		return ctrl.Result{}, utils.WrapError("Listing NamespaceLabels failed", err)
	}

	for _, nsLabel := range nsLabels {
		if !nsLabel.DeletionTimestamp.IsZero() {
			if err := finalizer.HandleFinalizer(ctx, r.Client, r.Logger, r.Recorder, &nsLabel); err != nil {
				return ctrl.Result{}, utils.WrapError("Handling finalizer failed", err)
			}
			continue
		}

		if err := finalizer.AddFinalizer(ctx, r.Client, r.Logger, &nsLabel); err != nil {
			return ctrl.Result{}, utils.WrapError("Adding finalizer failed", err)
		}

		combinedLabels := r.combineLabels(nsLabels)
		if err := r.applyLabels(ctx, req.Namespace, combinedLabels); err != nil {
			return r.handleLabelSyncError(ctx, nsLabels, err)
		}

		return r.handleSuccessfulReconciliation(ctx, nsLabels, req.Namespace)
	}

	return ctrl.Result{}, nil
}

func (r *NamespaceLabelReconciler) combineLabels(nsLabels []danateamv1.NamespaceLabel) map[string]string {
	combinedLabels := make(map[string]string)
	for _, nsLabel := range nsLabels {
		labelmanager.ApplyNamespaceLabel(nsLabel, combinedLabels, r.ProtectedLabels, r.Logger)
	}
	return combinedLabels
}

func (r *NamespaceLabelReconciler) applyLabels(ctx context.Context, namespaceName string, combinedLabels map[string]string) error {
	namespace, err := r.Client.GetNamespace(ctx, namespaceName)
	if err != nil {
		return utils.WrapError("Getting Namespace failed", err)
	}

	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	namespace.Labels = utils.MergeMaps(namespace.Labels, combinedLabels)
	if err := r.Client.UpdateNamespace(ctx, namespace); err != nil {
		return utils.WrapError("Updating Namespace with new labels failed", err)
	}

	r.Recorder.Eventf(namespace, corev1.EventTypeNormal, "LabelsApplied", "Labels applied successfully to namespace %s", namespace.Name)
	return nil
}

func (r *NamespaceLabelReconciler) handleSuccessfulReconciliation(ctx context.Context, nsLabels []danateamv1.NamespaceLabel, namespace string) (ctrl.Result, error) {
	for _, nsLabel := range nsLabels {
		status.UpdateCondition(&nsLabel.Status, "LabelsApplied", "True", "Success", "Labels applied successfully")
		nsLabel.Status.Applied = true
		nsLabel.Status.ObservedGeneration = nsLabel.Generation

		if err := r.Client.UpdateNamespaceLabelStatus(ctx, &nsLabel); err != nil {
			r.Logger.Error(err, "Failed to update NamespaceLabel status")
			return ctrl.Result{}, utils.WrapError("Updating NamespaceLabel status failed", err)
		}
	}

	r.Logger.Info("Successfully reconciled NamespaceLabel", "namespace", namespace)
	r.Recorder.Eventf(nil, corev1.EventTypeNormal, "ReconcileSuccess", "Successfully reconciled NamespaceLabel for namespace %s", namespace)
	return ctrl.Result{}, nil
}

func (r *NamespaceLabelReconciler) handleLabelSyncError(ctx context.Context, nsLabels []danateamv1.NamespaceLabel, err error) (ctrl.Result, error) {
	for _, nsLabel := range nsLabels {
		status.UpdateCondition(&nsLabel.Status, "LabelsApplied", "False", "Error", "Failed to apply labels")
		_ = r.Client.UpdateNamespaceLabelStatus(ctx, &nsLabel)
	}
	r.Logger.Error(err, "Failed to apply labels to the Namespace")
	r.Recorder.Eventf(nil, corev1.EventTypeWarning, "LabelSyncFailed", "Failed to apply labels to namespace: %v", err)
	return ctrl.Result{}, utils.WrapError("Applying labels failed", err)
}

func (r *NamespaceLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ProtectedLabels = labelmanager.LoadProtectedLabelsFromEnv()
	r.Logger = ctrl.Log.WithName("namespaceLabelController")
	r.Scheme = mgr.GetScheme()
	r.Recorder = mgr.GetEventRecorderFor("namespaceLabelController")

	return ctrl.NewControllerManagedBy(mgr).
		For(&danateamv1.NamespaceLabel{}).
		Complete(r)
}
