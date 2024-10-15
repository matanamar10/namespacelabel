package controller

import (
	"context"
	"sort"

	"github.com/go-logr/logr"
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	"github.com/matanamar10/namesapcelabel/internal/pkg/client"
	"github.com/matanamar10/namesapcelabel/internal/pkg/labelmanager"
	"github.com/matanamar10/namesapcelabel/internal/pkg/set"
	"github.com/matanamar10/namesapcelabel/internal/pkg/status"
	"github.com/matanamar10/namesapcelabel/internal/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors" // Correct import for error checking
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

type NamespaceLabelReconciler struct {
	Client          client.NamespaceLabelClient
	Logger          logr.Logger
	ProtectedLabels set.Set[string]
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder // EventRecorder for events
}

// Reconcile performs the reconciliation for NamespaceLabel resources.
func (r *NamespaceLabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsLabels, err := r.Client.ListNamespaceLabels(ctx, req.Namespace)
	if err != nil {
		// Record a warning event for failing to list NamespaceLabels
		r.Recorder.Eventf(nil, corev1.EventTypeWarning, "NamespaceLabelListFailed", "Failed to list NamespaceLabels for namespace %s: %v", req.Namespace, err)
		return ctrl.Result{}, err
	}

	namespace, err := r.Client.GetNamespace(ctx, req.Namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Recorder.Eventf(nil, corev1.EventTypeWarning, "NamespaceNotFound", "Namespace %s not found", req.Namespace)
			return ctrl.Result{}, nil
		}
		r.Recorder.Eventf(nil, corev1.EventTypeWarning, "NamespaceFetchFailed", "Failed to fetch Namespace %s: %v", req.Namespace, err)
		return ctrl.Result{}, err
	}

	combinedLabels := r.combineLabels(nsLabels)
	if err := r.applyLabels(ctx, namespace, combinedLabels); err != nil {
		r.Recorder.Eventf(namespace, corev1.EventTypeWarning, "LabelSyncFailed", "Failed to apply labels for namespace %s", req.Namespace)
		return r.handleLabelSyncError(ctx, nsLabels, err)
	}

	r.Recorder.Eventf(namespace, corev1.EventTypeNormal, "Reconciled", "Successfully reconciled Namespace %s", req.Namespace)
	return r.handleSuccessfulReconciliation(ctx, nsLabels, namespace)
}

func (r *NamespaceLabelReconciler) combineLabels(nsLabels []danateamv1.NamespaceLabel) map[string]string {
	combinedLabels := make(map[string]string)
	sortedLabels := r.sortNamespaceLabelsByTimestamp(nsLabels)

	for _, nsLabel := range sortedLabels {
		labelmanager.ApplyNamespaceLabel(nsLabel, combinedLabels, r.ProtectedLabels, r.Logger)
	}

	return combinedLabels
}

func (r *NamespaceLabelReconciler) sortNamespaceLabelsByTimestamp(nsLabels []danateamv1.NamespaceLabel) []danateamv1.NamespaceLabel {
	sort.Slice(nsLabels, func(i, j int) bool {
		return nsLabels[i].CreationTimestamp.After(nsLabels[j].CreationTimestamp.Time)
	})
	return nsLabels
}

func (r *NamespaceLabelReconciler) applyLabels(ctx context.Context, namespace *corev1.Namespace, combinedLabels map[string]string) error {
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	namespace.Labels = utils.MergeMaps(namespace.Labels, combinedLabels)

	err := r.Client.UpdateNamespace(ctx, namespace)
	if err != nil {
		r.Recorder.Eventf(namespace, corev1.EventTypeWarning, "LabelSyncFailed", "Failed to update labels for namespace %s: %v", namespace.Name, err)
		return err
	}

	r.Recorder.Eventf(namespace, corev1.EventTypeNormal, "LabelsApplied", "Successfully applied labels to namespace %s", namespace.Name)
	return nil
}

func (r *NamespaceLabelReconciler) handleSuccessfulReconciliation(ctx context.Context, nsLabels []danateamv1.NamespaceLabel, namespace *corev1.Namespace) (ctrl.Result, error) {
	for _, nsLabel := range nsLabels {
		status.UpdateCondition(&nsLabel.Status, "LabelsApplied", "True", "Success", "Labels applied successfully")
		nsLabel.Status.Applied = true
		nsLabel.Status.ObservedGeneration = nsLabel.Generation

		if err := r.Client.UpdateNamespaceLabelStatus(ctx, &nsLabel); err != nil {
			r.Logger.Error(err, "Failed to update NamespaceLabel status")
			return ctrl.Result{}, err
		}
	}

	r.Logger.Info("Successfully reconciled NamespaceLabel", "namespace", namespace.Name)
	return ctrl.Result{}, nil
}

func (r *NamespaceLabelReconciler) handleLabelSyncError(ctx context.Context, nsLabels []danateamv1.NamespaceLabel, err error) (ctrl.Result, error) {
	for _, nsLabel := range nsLabels {
		status.UpdateCondition(&nsLabel.Status, "LabelsApplied", "False", "Error", "Failed to apply labels")
		_ = r.Client.UpdateNamespaceLabelStatus(ctx, &nsLabel)
	}
	r.Logger.Error(err, "Failed to apply labels to the Namespace")
	r.Recorder.Eventf(nil, corev1.EventTypeWarning, "LabelSyncFailed", "Failed to apply labels for Namespace")
	return ctrl.Result{}, err
}

func (r *NamespaceLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ProtectedLabels = labelmanager.LoadProtectedLabelsFromEnv()
	r.Logger = ctrl.Log.WithName("namespaceLabelController")
	r.Scheme = mgr.GetScheme()
	r.Recorder = mgr.GetEventRecorderFor("namespaceLabelController") // Initialize EventRecorder

	return ctrl.NewControllerManagedBy(mgr).
		For(&danateamv1.NamespaceLabel{}).
		Complete(r)
}
