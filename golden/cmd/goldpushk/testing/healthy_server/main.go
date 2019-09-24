package main

// This is a dummy service used to test goldpushk. It should eventually reach the "Running" state on
// Kubernetes.

import (
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/go/httputils"
)

func main() {
	flagInstance := flag.String("instance", "", "Instance name.")
	flagPort := flag.Int("port", 0, "Port to listen to.")
	flag.Parse()
	if *flagInstance == "" {
		fmt.Println("You must provide flag --instance.")
		os.Exit(1)
	}
	if *flagPort == 0 {
		fmt.Println("You must provide flag --port.")
		os.Exit(1)
	}
	if *flagPort < 0 || *flagPort > 65535 {
		fmt.Println("Flag --port must be set to a value between 0 and 65535.")
		os.Exit(1)
	}

	fmt.Printf("Running goldpushk healthy_server (instance name: \"%s\") on port %d.\n", *flagInstance, *flagPort)
	httputils.RunHealthCheckServer(fmt.Sprintf(":%d", *flagPort))
}
