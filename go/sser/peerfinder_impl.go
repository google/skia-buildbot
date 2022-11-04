package sser

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PeerFinderImpl implements PeerFinder.
type PeerFinderImpl struct {
	clientset kubernetes.Interface

	// kubernetes namespace the pods are running in.
	namespace string

	// kubernetes label selector used to choose pods that are peers. For example: "app=skiaperf".
	labelSelector string

	// The most recent version of a kubernetes List response, used in Watch
	// requests.
	// https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
	resourceVersion string
}

// NewPeerFinder returns a new instance of PeerFinderImpl.
func NewPeerFinder(clientset kubernetes.Interface, namespace string, labelSelector string) (*PeerFinderImpl, error) {
	return &PeerFinderImpl{
		clientset:     clientset,
		namespace:     namespace,
		labelSelector: labelSelector,
	}, nil
}

func (p *PeerFinderImpl) findAllPeerPodIPAddresses(ctx context.Context) ([]string, string, error) {
	pods, err := p.clientset.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: p.labelSelector,
	})
	if err != nil {
		return nil, "", skerr.Wrapf(err, "Could not list peer pods")
	}

	urls := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		// Note that the PodIP can be the empty string.
		if p.Status.Phase == v1.PodRunning && p.Status.PodIP != "" {
			urls = append(urls, p.Status.PodIP)
		}
	}
	return urls, pods.ResourceVersion, nil
}

// Start implements PeerFinder.
func (p *PeerFinderImpl) Start(ctx context.Context) ([]string, <-chan []string, error) {
	var ips []string
	var err error
	ips, p.resourceVersion, err = p.findAllPeerPodIPAddresses(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "populating initial set of peers.")
	}

	ch := make(chan []string)

	// Now start a background Go routine that uses the kubernetes Watch feature
	// to keep track of updates to the peer pods.
	go func() {
		for {
			watch, err := p.clientset.CoreV1().Pods(p.namespace).Watch(ctx, metav1.ListOptions{
				LabelSelector:   p.labelSelector,
				ResourceVersion: p.resourceVersion,
			})
			if err != nil {
				sklog.Warningf("watch for peer changes failed: %s", err)
				p.resourceVersion = ""
			} else {
				watch.Stop()
			}
			if ctx.Err() != nil {
				sklog.Errorf("start Go routine cancelled: %s", err)
				close(ch)
				return
			}
			// Either our watch has failed, or it has returned successfully, and
			// in either case we should request a new set of IP addresses. Note
			// that this also updates the resourceVersion, which is needed for
			// the Watch to continue.
			newIps, newResourceVersion, err := p.findAllPeerPodIPAddresses(ctx)
			if err == nil {
				p.resourceVersion = newResourceVersion
				ch <- newIps
			} else {
				p.resourceVersion = ""
			}
		}
	}()

	return ips, ch, nil
}
