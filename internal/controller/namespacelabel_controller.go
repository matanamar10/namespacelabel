package controller

import (
	"context"
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type NamespaceLabelReconciler struct {
	client.Client
}

var protectedLabels = sets.NewString(
	"kubernetes.io/managed-by",
	"kubernetes.io/created-by",
	"control-plane",
	"cluster-owner",
)

func (r *NamespaceLabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("namespacelabel", req.NamespacedName)

	var nsLabel danateamv1.NamespaceLabel
	if err := r.Get(ctx, req.NamespacedName, &nsLabel); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("NamespaceLabel resource not found, may have been deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch NamespaceLabel")
		return ctrl.Result{}, err
	}

	var namespace corev1.Namespace
	if err := r.Get(ctx, types.NamespacedName{Name: nsLabel.Namespace}, &namespace); err != nil {
		logger.Error(err, "Failed to fetch Namespace", "namespace", nsLabel.Namespace)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.applyLabels(ctx, &namespace, &nsLabel); err != nil {
		logger.Error(err, "Failed to apply labels to the Namespace")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled NamespaceLabel", "namespace", namespace.Name)
	return ctrl.Result{}, nil
}

func (r *NamespaceLabelReconciler) applyLabels(ctx context.Context, namespace *corev1.Namespace, nsLabel *danateamv1.NamespaceLabel) error {
	logger := log.FromContext(ctx)

	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	for key, value := range nsLabel.Spec.Labels {
		if protectedLabels.Has(key) {
			logger.Info("Skipping protected label", "label", key)
			continue
		}
		namespace.Labels[key] = value
	}

	for key := range namespace.Labels {
		if _, found := nsLabel.Spec.Labels[key]; !found && !protectedLabels.Has(key) {
			delete(namespace.Labels, key)
		}
	}

	if err := r.Update(ctx, namespace); err != nil {
		logger.Error(err, "Failed to update Namespace with new labels")
		return err
	}

	logger.Info("Labels successfully synchronized with the namespace", "namespace", namespace.Name)
	return nil
}

func (r *NamespaceLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danateamv1.NamespaceLabel{}).
		Complete(r)
}
