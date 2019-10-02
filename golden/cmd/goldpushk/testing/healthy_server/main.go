package main

// This is a dummy service used to test goldpushk. It should eventually reach the "Running" state on
// Kubernetes.

import (
	"flag"
	"fmt"

	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

func main() {
	// Define a few parameters to test that the configuration file (.yaml) templating works.
	flagInstance := flag.String("instance", "", "Instance name.")
	flagPort := flag.Int("port", 0, "Port to listen to.")
	flagConfigFilename := flag.String("config_filename", "", "Configuration file in JSON5 format.")
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

	// Read configuration file.
	json5Config := struct {
		Salutation string
	}{}
	if err := config.ParseConfigFile(*flagConfigFilename, "config_filename", &json5Config); err != nil {
		sklog.Fatalf("Could not read configuration file %s: %s\n", *flagConfigFilename, err)
	}

	// Log the salutation read from the config file.
	sklog.Infof("Salutation: %s", json5Config.Salutation)

	// Log additional flags and start the health check server.
	sklog.Infof("Running goldpushk healthy_server (instance name: \"%s\") on port %d.\n", *flagInstance, *flagPort)
	httputils.RunHealthCheckServer(fmt.Sprintf(":%d", *flagPort))
}
