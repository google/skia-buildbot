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

	/*
			// This is how we should report 'dirty' daemonset status.

		Actually use the watcher/cache:

		  https://stackoverflow.com/questions/40975307/how-to-watch-events-on-a-kubernetes-service-using-its-go-client

			pods, err := d.clientSet.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
				FieldSelector: "status.phase=Running",
				// Watch: ..?
			})
			runningImage := pods.Items[0].Spec.Containers[0].Image
			ds, err := d.clientSet.AppsV1().DaemonSets("default").Get(ctx, "rpi-swarming", metav1.GetOptions{})
			ds, err := d.clientSet.AppsV1().DaemonSets("default").List(ctx, metav1.ListOptions{
				// Watch: ..?
			})

			wantedImage := ds.Spec.Template.Spec.Containers[0].Image
			if runningImage != wantedImage {
				// Report as needing pod deletion.
			}
	*/
}

var _ PodDeleter = (*Impl)(nil)
