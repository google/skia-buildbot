package main

import (
	"flag"
	"fmt"
	"net"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// flags
var (
	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	logging    = flag.Bool("logging", true, "Control logging.")
)

func singleStep(pod, port string) {
	fmt.Println("Begin")
	// First start a connection to sshd.
	conn, err := net.Dial("tcp", "localhost:22")
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(conn)

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// On the pod we run netcat in listen mode.
	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf("nc -vv -l -p %s", port),
	}

	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(pod).
		Namespace("default").SubResource("exec")

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

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		sklog.Fatal(err)
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  conn,
		Stdout: conn,
	})
	if err != nil {
		sklog.Fatalf("Failed: %s", err)
	}
}

func main() {
	common.InitWithMust(
		"testpf",
		common.SLogLoggingOpt(logging),
	)

	for {
		// Here we should ask switchboard/machines.skia.org for the pod name and port to use.
		// Also record the pod/port in machine.Description
		singleStep("switch-pod-1", "9000")
		// Here we should tell switchboard/machines.skia.org that we are done with the pod/port.
		// Also remove the pod/port from machine.Description
	}
}
