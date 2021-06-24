// Package revportforward establishes a reverse port-forward from a kubernetes
// pod to localhost.
//
// Setting up a port-forward from a kubernetes pod is simple:
//
//    $ kubectl port-forward mypod 8888:5000
//
// The above will setup a port-forward, i.e. it will listen on port 8888
// locally, forwarding the traffic to 5000 in the pod named "mypod".
//
// What is more involved is setting up a port-forward in the reverse direction,
// which this code does.
//
// Note that for this to work, netcat (nc) must be installed in the pod.
//
// The code works by running netcat (nc) in the pod in listen mode and then
// connects the exec streams to local target address.
//
// This also support having ncrev installed on the pod, a safer version of nc in
// that it checks that there are no other listeners on the given port before
// starting.
package revportforward

import (
	"context"
	"fmt"
	"net"
	"os"

	"go.skia.org/infra/go/skerr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

// ReversePortForward creates reverse port-forwards to pods running in a
// kubernetes cluster.
type ReversePortForward struct {
	config       *rest.Config
	localaddress string
	useNcRev     bool
}

// New returns a new RevPortForward instance.
//
// kubeconfig - The full name of the kubeconfig file.
// podName - The name of the pod found in the cluster pointed to by the kubeconfig file.
// podPort - The port to forward from within the pod.
// localaddress - The address we want the incoming connection to be forwarded
//    to, something like "localhost:22"
func New(kubeconfig, localaddress string, useNcRev bool) (*ReversePortForward, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to initialize from kubeconfig: %s", kubeconfig)
	}
	return &ReversePortForward{
		config:       config,
		localaddress: localaddress,
		useNcRev:     useNcRev,
	}, nil
}

// Start a reverse port-forward. This function does not return as long as an
// active connection exists.
//
// Note that as connections are made and then closed this function may return,
// so it should be called from within a loop, e.g.:
//
// for {
//    if err := rpf.Start(ctx); err != nil {
//		sklog.Error(err)
//	  }
// }
func (r *ReversePortForward) Start(ctx context.Context, podName string, podPort int) error {
	fmt.Println("Begin")
	// First start a connection to the localaddress.
	var d net.Dialer

	// If the Context is cancelled then this connection should close, which
	// should cause exec.Stream() below to exit, in theory, but it probably
	// won't: https://github.com/kubernetes/client-go/issues/554
	conn, err := d.DialContext(ctx, "tcp", r.localaddress)
	if err != nil {
		return skerr.Wrapf(err, "Failed to connect to localaddress: %s", r.localaddress)
	}

	// Then execute netcat (nc) on the pod.
	clientset, err := kubernetes.NewForConfig(r.config)
	if err != nil {
		panic(err)
	}
	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf("nc -vv -l -p %d", podPort),
	}
	if r.useNcRev {
		hostname, _ := os.Hostname() // If hostname errs then use the empty string.
		cmd = []string{
			"sh",
			"-c",
			fmt.Sprintf("ncrev --port=:%d --machine=%s", podPort, hostname),
		}
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace("default").SubResource("exec")
	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  false,
		TTY:     false,
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(r.config, "POST", req.URL())
	if err != nil {
		return skerr.Wrapf(err, "Failed to run netcat on the pod: %q", podName)
	}

	// exec.Stream will not return until the connection is broken.
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  conn,
		Stdout: conn,
	})
	if err != nil {
		return skerr.Wrapf(err, "Stream connection failed.")
	}
	return nil
}
