package client

import (
	"context"
	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceLabelClient defines the interface for operations related to Namespace and NamespaceLabel CRs.
type NamespaceLabelClient interface {
	GetNamespace(ctx context.Context, name string) (*corev1.Namespace, error)
	ListNamespaceLabels(ctx context.Context, namespace string) ([]danateamv1.NamespaceLabel, error)
	UpdateNamespace(ctx context.Context, namespace *corev1.Namespace) error
	UpdateNamespaceLabelStatus(ctx context.Context, label *danateamv1.NamespaceLabel) error
}

// KubernetesNamespaceLabelClient implements NamespaceLabelClient with Kubernetes API.
type KubernetesNamespaceLabelClient struct {
	client.Client
}

func (kc *KubernetesNamespaceLabelClient) GetNamespace(ctx context.Context, name string) (*corev1.Namespace, error) {
	var namespace corev1.Namespace
	err := kc.Get(ctx, client.ObjectKey{Name: name}, &namespace)
	return &namespace, err
}

func (kc *KubernetesNamespaceLabelClient) ListNamespaceLabels(ctx context.Context, namespace string) ([]danateamv1.NamespaceLabel, error) {
	var nsLabelList danateamv1.NamespaceLabelList
	err := kc.List(ctx, &nsLabelList, client.InNamespace(namespace))
	return nsLabelList.Items, err
}

func (kc *KubernetesNamespaceLabelClient) UpdateNamespace(ctx context.Context, namespace *corev1.Namespace) error {
	return kc.Update(ctx, namespace)
}

func (kc *KubernetesNamespaceLabelClient) UpdateNamespaceLabelStatus(ctx context.Context, label *danateamv1.NamespaceLabel) error {
	return kc.Status().Update(ctx, label)
}
