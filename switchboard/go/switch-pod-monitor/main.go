package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/switchboard"
)

var (
	configFlag = flag.String("config", "../machine/configs/test.json", "The path to the configuration file.")
	local      = flag.Bool("local", false, "Running locally if true, as opposed to running in GCE.")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	common.InitWithMust(
		"switch-pod-monitor",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	ctx := context.Background()

	// Load config file.
	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		sklog.Fatalf("Failed to open config file: %q: %s", *configFlag, err)
	}

	s, err := switchboard.New(ctx, *local, instanceConfig)
	if err != nil {
		sklog.Fatalf("Failed to initialize Switchboard instance: %s", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatalf("Failed to load hostname: %s", err)
	}
	if err := s.AddPod(ctx, hostname); err != nil {
		sklog.Fatal(err)
	}
	defer func() {
		if err := s.RemovePod(ctx, hostname); err != nil {
			sklog.Fatal(err)
		}
	}()

	consecutiveFailures := 0
	for range time.Tick(switchboard.PodKeepAliveDuration) {
		if err := s.KeepAlivePod(ctx, hostname); err != nil {
			consecutiveFailures++
			sklog.Errorf("Failed to keep pod alive: %s", err)
			if consecutiveFailures >= switchboard.PodMaxConsecutiveKeepAliveErrors {
				sklog.Fatalf("Exiting: Switchpod.KeepAlivePod failed %d consecutive times", consecutiveFailures)
			}
		}
	}
}
