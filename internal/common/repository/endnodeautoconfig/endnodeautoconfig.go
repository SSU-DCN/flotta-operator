package endnodeautoconfig

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/project-flotta/flotta-operator/api/v1alpha1"
)

//go:generate mockgen -package=endnodeautoconfig -destination=mock_endnodeautoconfig.go . Repository
type Repository interface {
	Read(ctx context.Context, name string, namespace string) (*v1alpha1.EndNodeAutoConfig, error)
	Create(ctx context.Context, edgeDevice *v1alpha1.EndNodeAutoConfig) error
	PatchStatus(ctx context.Context, endNodeAutoConfig *v1alpha1.EndNodeAutoConfig, patch *client.Patch) error
	Patch(ctx context.Context, old, new *v1alpha1.EndNodeAutoConfig) error
	ListForSelector(ctx context.Context, selector *metav1.LabelSelector, namespace string) ([]v1alpha1.EndNodeAutoConfig, error)
	ListByNamespace(ctx context.Context, namespace string) ([]*v1alpha1.EndNodeAutoConfig, error)
	GetByDevice(ctx context.Context, namespace, device string) (*v1alpha1.EndNodeAutoConfig, error)
	ListByDevice(ctx context.Context, namespace, device string) ([]*v1alpha1.EndNodeAutoConfig, error)

	RemoveFinalizer(ctx context.Context, endNodeAutoConfig *v1alpha1.EndNodeAutoConfig, finalizer string) error
}

type CRRepository struct {
	client client.Client
}

func NewEndNodeAutoConfigRepository(client client.Client) *CRRepository {
	return &CRRepository{client: client}
}

func (r *CRRepository) Read(ctx context.Context, name string, namespace string) (*v1alpha1.EndNodeAutoConfig, error) {
	endNodeAutoConfig := v1alpha1.EndNodeAutoConfig{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &endNodeAutoConfig)
	return &endNodeAutoConfig, err
}

func (r *CRRepository) Create(ctx context.Context, endNodeAutoConfig *v1alpha1.EndNodeAutoConfig) error {
	return r.client.Create(ctx, endNodeAutoConfig)
}

func (r *CRRepository) PatchStatus(ctx context.Context, endNodeAutoConfig *v1alpha1.EndNodeAutoConfig, patch *client.Patch) error {
	return r.client.Status().Patch(ctx, endNodeAutoConfig, *patch)
}

func (r *CRRepository) Patch(ctx context.Context, old, new *v1alpha1.EndNodeAutoConfig) error {
	patch := client.MergeFrom(old)
	return r.client.Patch(ctx, new, patch)
}

func (r *CRRepository) RemoveFinalizer(ctx context.Context, endNodeAutoConfig *v1alpha1.EndNodeAutoConfig, finalizer string) error {
	cp := endNodeAutoConfig.DeepCopy()

	var finalizers []string
	for _, f := range cp.Finalizers {
		if f != finalizer {
			finalizers = append(finalizers, f)
		}
	}
	cp.Finalizers = finalizers

	err := r.Patch(ctx, endNodeAutoConfig, cp)
	if err == nil {
		endNodeAutoConfig.Finalizers = cp.Finalizers
	}

	return nil
}

func (r CRRepository) ListForSelector(ctx context.Context, selector *metav1.LabelSelector, namespace string) ([]v1alpha1.EndNodeAutoConfig, error) {
	s, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	options := client.ListOptions{
		Namespace:     namespace,
		LabelSelector: s,
	}
	var edl v1alpha1.EndNodeAutoConfigList
	err = r.client.List(ctx, &edl, &options)
	if err != nil {
		return nil, err
	}

	return edl.Items, nil
}

// ReadEndNodeAutoConfigByNamespace reads the list of EndNodeAutoConfig resources from the given namespace.
func (r *CRRepository) ListByNamespace(ctx context.Context, namespace string) ([]*v1alpha1.EndNodeAutoConfig, error) {
	// Create a list to store the EndNodeAutoConfig resources.
	endNodeAutoConfigList := &v1alpha1.EndNodeAutoConfigList{}

	// Use the Kubernetes client to list all EndNodeAutoConfig resources in the specified namespace.
	err := r.client.List(ctx, endNodeAutoConfigList, client.InNamespace(namespace))

	if err != nil {
		return nil, err
	}

	// Convert the list of items in the EndNodeAutoConfigList to a slice of pointers to EndNodeAutoConfig objects.
	var endNodeAutoConfigs []*v1alpha1.EndNodeAutoConfig
	for _, item := range endNodeAutoConfigList.Items {
		endNodeAutoConfigs = append(endNodeAutoConfigs, &item)
	}

	// Return the list of EndNodeAutoConfig resources and nil for the error.
	return endNodeAutoConfigs, nil
}

func (r *CRRepository) GetByDevice(ctx context.Context, device string, namespace string) (*v1alpha1.EndNodeAutoConfig, error) {
	var endNodeAutoConfig *v1alpha1.EndNodeAutoConfig
	// fetch all configs in the namespace
	listEndNodeAutoConfigInNamespace, err := r.ListByNamespace(ctx, namespace)
	for _, item := range listEndNodeAutoConfigInNamespace {
		if item.Spec.Device == device {
			// Found the matching EndNodeAutoConfig resource, return it.
			endNodeAutoConfig = item
			break
		}
	}

	return endNodeAutoConfig, err
}

func (r *CRRepository) ListByDevice(ctx context.Context, device string, namespace string) ([]*v1alpha1.EndNodeAutoConfig, error) {
	// fetch all configs in the namespace
	listEndNodeAutoConfigInNamespace, err := r.ListByNamespace(ctx, namespace)

	return listEndNodeAutoConfigInNamespace, err
}
