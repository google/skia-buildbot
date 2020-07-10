package main

// This is a test service used to test goldpushk. It should eventually reach the "Running" state on
// Kubernetes.

import (
	"flag"
	"fmt"
	"math/rand"
	"time"

	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

func main() {
	// Define a few parameters to test that the configuration file (.yaml) templating works.
	flagInstance := flag.String("instance", "", "Instance name.")
	flagPort := flag.Int("port", 0, "Port to listen to.")
	flagConfigFilename := flag.String("config_filename", "", "Configuration file in JSON5 format.")
	usageActualDelayStr := "Actual delay will be a random duration between --min_delay and --max_delay."
	flagMinDelay := flag.Duration("min_delay", 0, "Minimum delay before launching the health check server (default: 0). "+usageActualDelayStr)
	flagMaxDelay := flag.Duration("max_delay", 0, "Minimum delay before launching the health check server (default: 0). "+usageActualDelayStr)
	flag.Parse()
	if *flagInstance == "" {
		sklog.Fatal("You must provide flag --instance.")
	}
	if *flagPort == 0 {
		sklog.Fatal("You must provide flag --port.")
	}
	if *flagPort < 0 || *flagPort > 65535 {
		sklog.Fatal("Flag --port must be set to a value between 0 and 65535.")
	}
	if *flagConfigFilename == "" {
		sklog.Fatal("You must provide flag --config_filename.")
	}
	if *flagMinDelay < 0 {
		sklog.Fatal("Flag --min_delay cannot be less than 0.")
	}
	if *flagMaxDelay < 0 {
		sklog.Fatal("Flag --max_delay cannot be less than 0.")
	}
	if *flagMinDelay > *flagMaxDelay {
		sklog.Fatalf("Flag --min_delay should be less than --max_delay (given values are %d and %d seconds, respectively).", *flagMinDelay/time.Second, *flagMaxDelay/time.Second)
	}

	// Read configuration file.
	json5Config := struct {
		Salutation string
	}{}
	if err := config.ParseConfigFile(*flagConfigFilename, "config_filename", &json5Config); err != nil {
		sklog.Fatalf("Could not read configuration file %s: %s\n", *flagConfigFilename, err)
	}

	// Log the salutation read from the config file.
	sklog.Infof("Salutation: %s", json5Config.Salutation)

	// Sleep for a random amount of time between --min_delay and --max_delay.
	rand.Seed(time.Now().UnixNano())
	delay := time.Duration(int(*flagMinDelay) + rand.Intn(int(*flagMaxDelay)-int(*flagMinDelay)+1))
	sklog.Infof("Sleeping for %d seconds before starting healthy_server.", delay/time.Second)
	time.Sleep(delay)

	// Log additional flags and start the health check server.
	sklog.Infof("Running goldpushk healthy_server (instance name: \"%s\") on port %d.\n", *flagInstance, *flagPort)
	httputils.RunHealthCheckServer(fmt.Sprintf(":%d", *flagPort))
}
