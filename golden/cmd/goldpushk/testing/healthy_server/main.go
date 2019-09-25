package main

// This is a dummy service used to test goldpushk. It should eventually reach the "Running" state on
// Kubernetes.

import (
	"flag"
	"fmt"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

func main() {
	// Define a few parameters to test that the configuration file (.yaml) templating works.
	flagInstance := flag.String("instance", "", "Instance name.")
	flagPort := flag.Int("port", 0, "Port to listen to.")
	flag.Parse()
	if *flagInstance == "" {
		sklog.Fatalf("You must provide flag --instance.\n")
	}
	if *flagPort == 0 {
		sklog.Fatalf("You must provide flag --port.\n")
	}
	if *flagPort < 0 || *flagPort > 65535 {
		sklog.Fatalf("Flag --port must be set to a value between 0 and 65535.\n")
	}

	sklog.Infof("Running goldpushk healthy_server (instance name: \"%s\") on port %d.\n", *flagInstance, *flagPort)
	httputils.RunHealthCheckServer(fmt.Sprintf(":%d", *flagPort))
}
