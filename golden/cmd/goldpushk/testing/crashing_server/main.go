package main

// This is a test service used to test goldpushk. It should get stuck in a "CrashLoopBackOff" state
// on Kubernetes.

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

func main() {
	// Define a few parameters to test that the configuration file (.yaml) templating works.
	flagInstance := flag.String("instance", "", "Instance name.")
	flagPort := flag.Int("port", 0, "Port to listen to.")
	flagCrashAfterSeconds := flag.Int64("crash_after_seconds", 0, "Time (in seconds) to wait before crashing.")
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
	if *flagCrashAfterSeconds == 0 {
		sklog.Fatalf("You must provide flag --crash_after_seconds.\n")
	}
	if *flagCrashAfterSeconds < 1 {
		sklog.Fatalf("Flag --crash_after_seconds must be set to a value greater or equal than 1.\n")
	}

	sklog.Infof("Running goldpushk crashing_server (instance name: \"%s\") on port %d. This process will end with exit code 1 in %d seconds.\n", *flagInstance, *flagPort, *flagCrashAfterSeconds)
	go httputils.RunHealthCheckServer(fmt.Sprintf(":%d", *flagPort))
	time.Sleep(time.Duration(*flagCrashAfterSeconds) * time.Second)
	sklog.Infof("Exiting with code 1 after %d seconds.", *flagCrashAfterSeconds)
	os.Exit(1)
}
