// Package main implements the revportforward cli. See the Usage function below for details.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/revportforward"
	"go.skia.org/infra/go/sklog"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// flags
var (
	kubeconfig   = flag.String("kubeconfig", "", "The absolute path to the kubeconfig file.")
	logging      = flag.Bool("logging", true, "Control logging.")
	podName      = flag.String("pod_name", "", "Name of the pod to reverse port-forward from.")
	podPort      = flag.Int("pod_port", -1, "The port on the pod.")
	localAddress = flag.String("local_address", "", `The address to forward the connection to, for example: "localhost:22".`)
	useNcRev     = flag.Bool("use_ncrev", true, "Use ncrev instead of netcat (nc).")
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s <flags>\n\n", os.Args[0])
		fmt.Printf(`revportforward make a reverse port-forward from a k8s pod to local address.

Setting up a port-forward from a kubernetes pod is simple:

   $ kubectl port-forward mypod 8888:5000

The above will setup a port-forward, i.e. it will listen on port 8888
locally, forwarding the traffic to 5000 in the pod named "mypod".

What is more involved is setting up a port-forward in the reverse direction,
which this application does. Note that for this application to work, netcat
(nc) must be installed in the pod.

Example:

   $ revportforward --kubeconfig=$HOME/.kube/config \
     --pod_name=switch-pod-2 \
	 --pod_port=9000 \
	 --local_address=localhost:22

Which will set up a connection from switch-pod-2:9000 to localhost:22,
which means an app running in switch-pod-2 can ssh to your desktop
machine simply by running:

    $ ssh -p 9000 127.0.0.1

Flags:
`)
		flag.PrintDefaults()
	}
}

func exitWithUsageAndMessage(msg string) {
	fmt.Println(msg)
	flag.Usage()
	os.Exit(1)
}

func main() {
	common.InitWithMust(
		"revportforward",
		common.SLogLoggingOpt(logging),
	)
	if *kubeconfig == "" {
		exitWithUsageAndMessage("The --kubeconfig flag is required.")
	}
	if *podName == "" {
		exitWithUsageAndMessage("The --pod_name flag is required.")
	}
	if *podPort == -1 {
		exitWithUsageAndMessage("The --pod_port flag is required.")
	}
	if *localAddress == "" {
		exitWithUsageAndMessage("The --local_address flag is required.")
	}

	r, err := revportforward.New(*kubeconfig, *podName, *podPort, *localAddress, *useNcRev)
	if err != nil {
		sklog.Fatal(err)
	}
	ctx := context.Background()
	for {
		sklog.Info("Starting connection.")
		if err := r.Start(ctx); err != nil {
			sklog.Error(err)
		}
	}
}
