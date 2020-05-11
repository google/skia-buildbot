package deleter

import (
	"context"

	"go.skia.org/infra/go/skerr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Impl implements PodDeleter.
type Impl struct {
	clientSet kubernetes.Interface
}

// New returns a new implementation of PodDeleter.
func New() (*Impl, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get in-cluster config.")
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get in-cluster clientset.")
	}
	return &Impl{
		clientSet: clientSet,
	}, nil
}

// Delete implements PodDeleter.
func (d *Impl) Delete(ctx context.Context, podName string) error {
	zero := int64(0)
	if err := d.clientSet.CoreV1().Pods("default").Delete(ctx, podName, metav1.DeleteOptions{
		GracePeriodSeconds: &zero,
	}); err != nil {
		return skerr.Wrapf(err, "Failed to delete pod: %q", podName)
	}
	return nil
}

var _ PodDeleter = (*Impl)(nil)
