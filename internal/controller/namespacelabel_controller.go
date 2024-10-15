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

// NamespaceLabelReconciler reconciles a NamespaceLabel object.
type NamespaceLabelReconciler struct {
	Client          client.NamespaceLabelClient
	Logger          logr.Logger
	ProtectedLabels set.Set[string]
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
}

// Reconcile is the core function that performs reconciliation for NamespaceLabel resources.
func (r *NamespaceLabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsLabels, err := r.Client.ListNamespaceLabels(ctx, req.Namespace)
	if err != nil {
		return r.logEventAndReturnError(nil, "NamespaceLabelListFailed", "Failed to list NamespaceLabels", err)
	}

	combinedLabels := r.combineLabels(nsLabels)

	for _, nsLabel := range nsLabels {
		if !nsLabel.DeletionTimestamp.IsZero() {
			if err := r.handleFinalizer(ctx, nsLabel); err != nil {
				return ctrl.Result{}, err
			}
			continue
		}

		if err := finalizer.AddFinalizer(ctx, r.Client, r.Logger, &nsLabel); err != nil {
			return r.logEventAndReturnError(nil, "FinalizerAddFailed", "Failed to add finalizer", err)
		}

		if err := r.applyLabels(ctx, req.Namespace, combinedLabels); err != nil {
			return r.logEventAndReturnError(nil, "ApplyLabelsFailed", "Failed to apply labels to namespace", err)
		}

		if err := r.updateStatus(ctx, nsLabel); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// combineLabels consolidates all labels from the NamespaceLabel objects.
func (r *NamespaceLabelReconciler) combineLabels(nsLabels []danateamv1.NamespaceLabel) map[string]string {
	combinedLabels := make(map[string]string)
	for _, nsLabel := range nsLabels {
		// ApplyNamespaceLabel directly without LabelManager instance
		labelmanager.ApplyNamespaceLabel(nsLabel, combinedLabels, r.ProtectedLabels, r.Logger)
	}
	return combinedLabels
}

// applyLabels applies the combined labels to the target namespace.
func (r *NamespaceLabelReconciler) applyLabels(ctx context.Context, namespaceName string, combinedLabels map[string]string) error {
	namespace, err := r.Client.GetNamespace(ctx, namespaceName)
	if err != nil {
		return utils.WrapError("Getting Namespace failed", err)
	}

	namespace.Labels = utils.MergeMaps(namespace.Labels, combinedLabels)
	if err := r.Client.UpdateNamespace(ctx, namespace); err != nil {
		return utils.WrapError("Updating Namespace with new labels failed", err)
	}

	r.Recorder.Eventf(namespace, corev1.EventTypeNormal, "LabelsApplied", "Labels applied successfully to namespace %s", namespace.Name)
	return nil
}

// updateStatus updates the status of a NamespaceLabel object after successful label application.
func (r *NamespaceLabelReconciler) updateStatus(ctx context.Context, nsLabel danateamv1.NamespaceLabel) error {
	status.UpdateCondition(&nsLabel.Status, "LabelsApplied", "True", "Success", "Labels applied successfully")
	nsLabel.Status.Applied = true
	nsLabel.Status.ObservedGeneration = nsLabel.Generation

	if err := r.Client.UpdateNamespaceLabelStatus(ctx, &nsLabel); err != nil {
		r.Logger.Error(err, "Failed to update NamespaceLabel status")
		return utils.WrapError("Updating NamespaceLabel status failed", err)
	}
	return nil
}

// handleFinalizer processes finalizer logic for a NamespaceLabel resource.
func (r *NamespaceLabelReconciler) handleFinalizer(ctx context.Context, nsLabel danateamv1.NamespaceLabel) error {
	return finalizer.HandleFinalizer(ctx, r.Client, r.Logger, r.Recorder, &nsLabel)
}

// logEventAndReturnError logs an error, records an event, and returns the wrapped error.
func (r *NamespaceLabelReconciler) logEventAndReturnError(obj runtime.Object, reason, message string, err error) (ctrl.Result, error) {
	r.Logger.Error(err, message)
	r.Recorder.Eventf(obj, corev1.EventTypeWarning, reason, message+": %v", err)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ProtectedLabels = labelmanager.LoadProtectedLabelsFromEnv() // Directly load the labels
	r.Logger = ctrl.Log.WithName("namespaceLabelController")
	r.Scheme = mgr.GetScheme()
	r.Recorder = mgr.GetEventRecorderFor("namespaceLabelController")

	return ctrl.NewControllerManagedBy(mgr).
		For(&danateamv1.NamespaceLabel{}).
		Complete(r)
}
