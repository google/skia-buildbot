// powercycle_server_ansible is an application that watches the machine server
// and powercycles test machines that need powercycling.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	"go.skia.org/infra/skolo/go/powercycle"
	"go.skia.org/infra/skolo/sys"
)

var (
	// Version can be changed via -ldflags.
	Version = "development"
)

// Flags
var (
	configFlag               = flag.String("config", "", "The name of the configuration file.")
	local                    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	powercycleConfigFilename = flag.String("powercycle_config", "", "The name of the config file for powercycle.Controller.")
	promPort                 = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	machineServerHost        = flag.String("machine_server", "https://machines.skia.org", "A URL with the scheme and domain name of the machine hosting the machine server API.")
)

var (
	// urlExpansionRegex is used to replace gorilla mux URL variables with
	// values.
	urlExpansionRegex = regexp.MustCompile("{.*}")
)

func main() {
	common.InitWithMust(
		"powercycle_server_ansible",
		common.PrometheusOpt(promPort),
		common.CloudLogging(local, "skia-public"),
	)
	sklog.Infof("Version: %s", Version)
	ctx := context.Background()

	if *powercycleConfigFilename == "" {
		sklog.Fatal("--powercycle_config flag must be supplied.")
	}

	if *configFlag == "" {
		sklog.Fatal("--config flag must be supplied.")
	}

	// Construct store.
	var instanceConfig config.InstanceConfig
	b, err := fs.ReadFile(configs.Configs, *configFlag)
	if err != nil {
		sklog.Fatalf("Failed to read config file %q: %s", *configFlag, err)
	}
	err = json.Unmarshal(b, &instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}

	// Construct powercycle controller.
	powerCycleConfigBytes, err := fs.ReadFile(sys.Sys, *powercycleConfigFilename)
	if err != nil {
		sklog.Fatalf("Failed to read config file %q: %s", *powercycleConfigFilename, err)
	}

	sklog.Info("Building powercycle.Controller from %q", *powercycleConfigFilename)
	powercycleController, err := powercycle.ControllerFromJSON5Bytes(ctx, powerCycleConfigBytes, true)
	if err != nil {
		sklog.Fatalf("Failed to instantiate powercycle.Controller: %s", err)
	}

	ts, err := auth.NewDefaultTokenSource(*local, "email")
	if err != nil {
		sklog.Fatalf("Failed to create tokensource: %s", err)
	}

	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().WithoutRetries().Client()

	watchForPowerCycle(ctx, httpClient, *machineServerHost, powercycleController)

	select {}
}

// watchForPowerCycle loops forever powercycling machines, this function does
// not return.
func watchForPowerCycle(ctx context.Context, httpClient *http.Client, machineServer string, powercycleController powercycle.Controller) {
	stepFailure := metrics2.GetCounter("powercycle_server_ansible_step_failure")
	stepSuccess := metrics2.GetCounter("powercycle_server_ansible_step_success")
	deviceIDs := powercycleController.DeviceIDs()

	for range time.Tick(time.Second * 5) {
		if err := singleStep(ctx, httpClient, machineServer, deviceIDs, powercycleController); err != nil {
			sklog.Errorf("Failed a singleStep of powercycle: %s", err)
			stepFailure.Inc(1)
		} else {
			stepSuccess.Inc(1)
		}
	}
}

// in returns true if machineID is in the list of deviceIDs.
func in(machineID string, deviceIDs []powercycle.DeviceID) bool {
	ret := false
	for _, deviceID := range deviceIDs {
		if string(deviceID) == machineID {
			return true
		}
	}
	return ret
}

// singleStep does a single round of requesting the list of devices to
// powercycle and attempts to powercycle each machine id in the
// powercycleControllers purview.
func singleStep(ctx context.Context, httpClient *http.Client, machineServer string, deviceIDs []powercycle.DeviceID, powercycleController powercycle.Controller) error {
	ret := []error{}
	u, err := url.Parse(machineServer)
	if err != nil {
		return skerr.Wrapf(err, "Failed to parse machineserver flag: %s", machineServer)
	}
	u.Path = rpc.PowerCycleListURL
	resp, err := httpClient.Get(u.String())
	if err != nil {
		return skerr.Wrapf(err, "Failed to retrieve list of devices needing powercycling.")
	}
	defer util.Close(resp.Body)
	var list rpc.ListPowerCycleResponse
	err = json.NewDecoder(resp.Body).Decode(&list)
	if err != nil {
		return skerr.Wrapf(err, "Failed to decode list of devices needing powercycling.")
	}
	for _, machineID := range list {
		if !in(machineID, deviceIDs) {
			continue
		}
		if err := powercycleController.PowerCycle(ctx, powercycle.DeviceID(machineID), 0); err != nil {
			ret = append(ret, skerr.Wrapf(err, "Failed to powercycle %q", machineID))
			continue
		} else {
			sklog.Infof("Successfully powercycled: %q", machineID)
		}

		// We expand rpc.PowerCycleCompleteURL with machineID.
		u.Path = urlExpansionRegex.ReplaceAllLiteralString(rpc.PowerCycleCompleteURL, machineID)
		_, err = httpClient.Post(u.String(), "text/plain", nil)
		if err != nil {
			ret = append(ret, skerr.Wrapf(err, "Failed to update machines after powercycling: %q", machineID))
		}
	}
	if len(ret) > 0 {
		return ret[0]
	}
	return nil
}
