package main

// This is a dummy service used to test goldpushk. It should get stuck in a "CrashLoopBackOff" state
// on Kubernetes.

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go.skia.org/infra/go/httputils"
)

func main() {
	flagInstance := flag.String("instance", "", "Instance name.")
	flagPort := flag.Int("port", 0, "Port to listen to.")
	flagCrashAfterSeconds := flag.Int64("crash_after_seconds", 0, "Time (in seconds) to wait before crashing.")
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
	if *flagCrashAfterSeconds == 0 {
		fmt.Println("You must provide flag --crash_after_seconds.")
		os.Exit(1)
	}
	if *flagCrashAfterSeconds < 1 {
		fmt.Println("Flag --crash_after_seconds must be set to a value greater or equal than 1.")
		os.Exit(1)
	}

	fmt.Printf("Running goldpushk crashing_server (instance name: \"%s\") on port %d. This process will end with exit code 1 in %d seconds.\n", *flagInstance, *flagPort, *flagCrashAfterSeconds)
	go httputils.RunHealthCheckServer(fmt.Sprintf(":%d", *flagPort))
	time.Sleep(time.Duration(*flagCrashAfterSeconds) * time.Second)
	os.Exit(1)
}
