package controller

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	webappv1 "my.domain/guestbook/api/v1" // Replace with your module path
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceLabelReconciler reconciles a NamespaceLabel object
type NamespaceLabelReconciler struct {
	client.Client
	Log    ctrl.Logger
	Scheme *runtime.Scheme
}

// Define management labels that should be protected from tenant modifications
var protectedLabels = sets.NewString("kubernetes.io/managed-by", "control-plane", "cluster-owner")

// Reconcile is the core logic that ensures NamespaceLabel resources sync with Namespace labels
func (r *NamespaceLabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespacelabel", req.NamespacedName)

	// Fetch the NamespaceLabel instance
	var nsLabel webappv1.NamespaceLabel
	if err := r.Get(ctx, req.NamespacedName, &nsLabel); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, it may have been deleted. Don't requeue.
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch NamespaceLabel")
		return ctrl.Result{}, err
	}

	// Fetch the corresponding namespace
	var namespace corev1.Namespace
	if err := r.Get(ctx, types.NamespacedName{Name: nsLabel.Name}, &namespace); err != nil {
		log.Error(err, "unable to fetch Namespace")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Apply the tenant's labels while protecting management labels
	if err := r.applyLabels(ctx, &namespace, &nsLabel); err != nil {
		log.Error(err, "failed to apply labels to the namespace")
		r.setCondition(&nsLabel.Status, "LabelsApplied", "False", "UpdateFailed", "Failed to apply labels")
		_ = r.Status().Update(ctx, &nsLabel)
		return ctrl.Result{}, err
	}

	// Update status to reflect successful reconciliation
	r.setCondition(&nsLabel.Status, "LabelsApplied", "True", "Success", "Labels successfully applied")
	nsLabel.Status.Applied = true
	nsLabel.Status.ObservedGeneration = nsLabel.Generation

	// Update the NamespaceLabel status
	if err := r.Status().Update(ctx, &nsLabel); err != nil {
		log.Error(err, "failed to update NamespaceLabel status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully applied labels to the namespace", "namespace", nsLabel.Name)
	return ctrl.Result{}, nil
}

// applyLabels applies tenant's labels from NamespaceLabel to Namespace while protecting management labels
func (r *NamespaceLabelReconciler) applyLabels(ctx context.Context, namespace *corev1.Namespace, nsLabel *webappv1.NamespaceLabel) error {
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	// Apply tenant labels while protecting management labels
	for key, value := range nsLabel.Spec.Labels {
		if protectedLabels.Has(key) {
			r.Log.Info("Skipping protected label", "label", key)
			continue
		}
		namespace.Labels[key] = value
	}

	// Clean up any labels that were removed from NamespaceLabel CRD
	for key := range namespace.Labels {
		if _, ok := nsLabel.Spec.Labels[key]; !ok && !protectedLabels.Has(key) {
			delete(namespace.Labels, key)
		}
	}

	// Update the namespace with the new labels
	if err := r.Update(ctx, namespace); err != nil {
		return err
	}

	return nil
}

// Helper method to set conditions in the status field
func (r *NamespaceLabelReconciler) setCondition(status *webappv1.NamespaceLabelStatus, conditionType, statusValue, reason, message string) {
	now := metav1.NewTime(time.Now())
	for i, condition := range status.Conditions {
		if condition.Type == conditionType {
			// Update existing condition
			status.Conditions[i].Status = statusValue
			status.Conditions[i].Reason = reason
			status.Conditions[i].Message = message
			status.Conditions[i].LastTransitionTime = now
			return
		}
	}
	// Add new condition if it doesn't exist
	newCondition := webappv1.Condition{
		Type:               conditionType,
		Status:             statusValue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	}
	status.Conditions = append(status.Conditions, newCondition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.NamespaceLabel{}).
		Complete(r)
}
