package controller

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Set[T comparable] map[T]struct{}

// NewSet creates a new generic set.
func NewSet[T comparable]() Set[T] {
	return make(Set[T])
}

// Add adds a value to the Set.
func (s Set[T]) Add(value T) {
	s[value] = struct{}{}
}

// Contains checks if a value exists in the Set.
func (s Set[T]) Contains(value T) bool {
	_, exists := s[value]
	return exists
}

// NamespaceLabelReconciler reconciles a NamespaceLabel object.
type NamespaceLabelReconciler struct {
	client.Client
	Logger          logr.Logger
	ProtectedLabels Set[string] // Protected labels loaded from environment variables
}

func loadProtectedLabelsFromEnv() Set[string] {
	labels := os.Getenv("PROTECTED_LABELS")
	if labels == "" {
		labels = "kubernetes.io/managed-by,kubernetes.io/created-by,control-plane,cluster-owner" // Default protected labels
	}

	protectedSet := NewSet[string]()
	for _, label := range strings.Split(labels, ",") {
		protectedSet.Add(label)
	}
	return protectedSet
}

func (r *NamespaceLabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsLabel, err := r.getNamespaceLabel(ctx, req)
	if err != nil {
		return r.handleGetNamespaceLabelError(err)
	}

	namespace, err := r.getNamespace(ctx, nsLabel.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.applyLabels(ctx, namespace, nsLabel); err != nil {
		return r.handleLabelSyncError(ctx, nsLabel, err)
	}

	return r.handleSuccessfulReconciliation(ctx, nsLabel, namespace)
}

func (r *NamespaceLabelReconciler) getNamespaceLabel(ctx context.Context, req ctrl.Request) (*danateamv1.NamespaceLabel, error) {
	logger := log.FromContext(ctx)
	var nsLabel danateamv1.NamespaceLabel
	err := r.Get(ctx, req.NamespacedName, &nsLabel)
	if err != nil {
		logger.Error(err, "Failed to fetch NamespaceLabel")
	}
	return &nsLabel, err
}

func (r *NamespaceLabelReconciler) getNamespace(ctx context.Context, namespaceName string) (*corev1.Namespace, error) {
	var namespace corev1.Namespace
	err := r.Get(ctx, types.NamespacedName{Name: namespaceName}, &namespace)
	if err != nil {
		r.Logger.Error(err, "Failed to fetch Namespace", "namespace", namespaceName)
		return nil, client.IgnoreNotFound(err)
	}
	return &namespace, nil
}

func (r *NamespaceLabelReconciler) applyLabels(ctx context.Context, namespace *corev1.Namespace, nsLabel *danateamv1.NamespaceLabel) error {
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	for key, value := range nsLabel.Spec.Labels {
		if !r.ProtectedLabels.Contains(key) {
			namespace.Labels[key] = value
		} else {
			r.Logger.Info("Skipping protected label", "label", key)
		}
	}

	for key := range namespace.Labels {
		if _, found := nsLabel.Spec.Labels[key]; !found && !r.ProtectedLabels.Contains(key) {
			delete(namespace.Labels, key)
		}
	}

	if err := r.Update(ctx, namespace); err != nil {
		r.Logger.Error(err, "Failed to update Namespace with new labels")
		return err
	}

	r.Logger.Info("Labels successfully synchronized with the namespace", "namespace", namespace.Name)
	return nil
}

func (r *NamespaceLabelReconciler) handleGetNamespaceLabelError(err error) (ctrl.Result, error) {
	if errors.IsNotFound(err) {
		r.Logger.Info("NamespaceLabel resource not found, may have been deleted")
		return ctrl.Result{}, nil
	}
	r.Logger.Error(err, "Failed to fetch NamespaceLabel")
	return ctrl.Result{}, err
}

func (r *NamespaceLabelReconciler) handleLabelSyncError(ctx context.Context, nsLabel *danateamv1.NamespaceLabel, err error) (ctrl.Result, error) {
	r.updateCondition(&nsLabel.Status, "LabelsApplied", "False", "Error", "Failed to apply labels")
	r.Logger.Error(err, "Failed to apply labels to the Namespace")
	_ = r.Status().Update(ctx, nsLabel)
	return ctrl.Result{}, err
}

func (r *NamespaceLabelReconciler) handleSuccessfulReconciliation(ctx context.Context, nsLabel *danateamv1.NamespaceLabel, namespace *corev1.Namespace) (ctrl.Result, error) {
	r.updateCondition(&nsLabel.Status, "LabelsApplied", "True", "Success", "Labels applied successfully")
	nsLabel.Status.Applied = true
	nsLabel.Status.ObservedGeneration = nsLabel.Generation

	if err := r.Status().Update(ctx, nsLabel); err != nil {
		r.Logger.Error(err, "Failed to update NamespaceLabel status")
		return ctrl.Result{}, err
	}

	r.Logger.Info("Successfully reconciled NamespaceLabel", "namespace", namespace.Name)
	return ctrl.Result{}, nil
}

func (r *NamespaceLabelReconciler) updateCondition(status *danateamv1.NamespaceLabelStatus, conditionType, statusValue, reason, message string) {
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

func (r *NamespaceLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ProtectedLabels = loadProtectedLabelsFromEnv()
	r.Logger = ctrl.Log.WithName("namespaceLabelController")
	return ctrl.NewControllerManagedBy(mgr).
		For(&danateamv1.NamespaceLabel{}).
		Complete(r)
}
