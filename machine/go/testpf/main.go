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

func singleStep() {
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
		"nc -vv -l -p 9000",
	}

	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name("switch-pod-1").
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
		singleStep()
	}
}
