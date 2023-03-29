package k8s

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"go.skia.org/infra/go/kube/clusterconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// Client provides an interface for interacting with a Kubernetes cluster. It is
// a thin wrapper around the Kubernetes API which allows for mocking requests.
type Client interface {
	// ListNamespaces retrieves all namespaces in the cluster.
	ListNamespaces(ctx context.Context, opts metav1.ListOptions) ([]corev1.Namespace, error)

	// ListPods retrieves all pods in the namespace.
	ListPods(ctx context.Context, namespace string, opts metav1.ListOptions) ([]corev1.Pod, error)

	// DeletePod deletes a pod.
	DeletePod(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error

	// ListStatefulSets retrieves all stateful sets in the namespace.
	ListStatefulSets(ctx context.Context, namespace string, opts metav1.ListOptions) ([]appsv1.StatefulSet, error)

	// GetStatefulSet retrieves a single StatefulSet.
	GetStatefulSet(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*appsv1.StatefulSet, error)
}

// ClientImpl implements Client.
type ClientImpl struct {
	c *kubernetes.Clientset
}

// NewClient returns a ClientImpl instance.
// TODO(borenet): Handle more of the setup here.
func NewClient(ctx context.Context, clientset *kubernetes.Clientset) (*ClientImpl, error) {
	return &ClientImpl{
		c: clientset,
	}, nil
}

// GetNamespaces implements Client.
func (c *ClientImpl) ListNamespaces(ctx context.Context, opts metav1.ListOptions) ([]corev1.Namespace, error) {
	result, err := c.c.CoreV1().Namespaces().List(ctx, opts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return result.Items, nil
}

// GetPods implements Client.
func (c *ClientImpl) ListPods(ctx context.Context, namespace string, opts metav1.ListOptions) ([]corev1.Pod, error) {
	result, err := c.c.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return result.Items, nil
}

// DeletePod implements Client.
func (c *ClientImpl) DeletePod(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return skerr.Wrap(c.c.CoreV1().Pods(namespace).Delete(ctx, name, opts))
}

// ListStatefulSets implements Client.
func (c *ClientImpl) ListStatefulSets(ctx context.Context, namespace string, opts metav1.ListOptions) ([]appsv1.StatefulSet, error) {
	result, err := c.c.AppsV1().StatefulSets(namespace).List(ctx, opts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return result.Items, nil
}

// GetStatefulSet implements Client.
func (c *ClientImpl) GetStatefulSet(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*appsv1.StatefulSet, error) {
	result, err := c.c.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return result, nil
}

// Assert that ClientImpl implements Client.
var _ Client = &ClientImpl{}

// NewInClusterClient creates a Client which can be used to interact with the
// cluster in which this service is running.
func NewInClusterClient(ctx context.Context) (*ClientImpl, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get in-cluster config")
	}
	sklog.Infof("Auth username: %s", config.Username)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get in-cluster clientset")
	}
	return NewClient(ctx, clientset)
}

// NewLocalClient uses the local kubeconfig file, the clusters config.json file,
// and a cluster name to create a Client which can be used to interact with that
// cluster.
// TODO(borenet): This requires that the user has previously created credentials
// for the requested cluster.
func NewLocalClient(ctx context.Context, kubeConfigFile, clusterConfigFile, cluster string) (*ClientImpl, error) {
	clusterConfig, err := clusterconfig.New(clusterConfigFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	clusterInfo, ok := clusterConfig.Clusters[cluster]
	if !ok {
		return nil, skerr.Fmt("unknown cluster %q", cluster)
	}
	cfg, err := clientcmd.LoadFromFile(kubeConfigFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cfg.CurrentContext = clusterInfo.ContextName
	clusterCfg, ok := cfg.Clusters[clusterInfo.ContextName]
	if !ok {
		return nil, skerr.Fmt("no cluster config for %q", clusterInfo.ContextName)
	}
	restCfg, err := clientcmd.BuildConfigFromKubeconfigGetter(clusterCfg.Server, func() (*api.Config, error) {
		return cfg, nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return NewClient(ctx, clientset)
}
