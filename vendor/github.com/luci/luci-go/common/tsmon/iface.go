// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/auth"
	gcps "github.com/luci/luci-go/common/gcloud/pubsub"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/tsmon/monitor"
	"github.com/luci/luci-go/common/tsmon/store"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/common/tsmon/types"
)

// Store returns the global metric store that contains all the metric values for
// this process.  Applications shouldn't need to access this directly - instead
// use the metric objects which provide type-safe accessors.
func Store(c context.Context) store.Store {
	return GetState(c).S
}

// Monitor returns the global monitor that sends metrics to monitoring
// endpoints.  Defaults to a nil monitor, but changed by InitializeFromFlags.
func Monitor(c context.Context) monitor.Monitor {
	return GetState(c).M
}

// Register is called by metric objects to register themselves.  This will panic
// if another metric with the same name is already registered.
func Register(c context.Context, m types.Metric) {
	state := GetState(c)
	state.RegisteredMetricsLock.Lock()
	defer state.RegisteredMetricsLock.Unlock()

	if _, ok := state.RegisteredMetrics[m.Info().Name]; ok {
		panic(fmt.Sprintf("A metric with the name '%s' was already registered", m.Info().Name))
	}

	state.RegisteredMetrics[m.Info().Name] = m

	if Store(c) != nil {
		Store(c).Register(m)
	}
}

// Unregister is called by metric objects to unregister themselves.
func Unregister(c context.Context, m types.Metric) {
	state := GetState(c)
	state.RegisteredMetricsLock.Lock()
	defer state.RegisteredMetricsLock.Unlock()

	delete(state.RegisteredMetrics, m.Info().Name)

	if Store(c) != nil {
		Store(c).Unregister(m)
	}
}

// SetStore changes the global metric store.  All metrics that were registered
// with the old store will be re-registered on the new store.
func SetStore(c context.Context, s store.Store) {
	GetState(c).SetStore(s)
}

// InitializeFromFlags configures the tsmon library from flag values.
//
// This will set a Target (information about what's reporting metrics) and a
// Monitor (where to send the metrics to).
func InitializeFromFlags(c context.Context, fl *Flags) error {
	// Load the config file, and override its values with flags.
	config, _ := loadConfig(fl.ConfigFile)

	if fl.Endpoint != "" {
		config.Endpoint = fl.Endpoint
	}
	if fl.Credentials != "" {
		config.Credentials = fl.Credentials
	}

	mon, err := initMonitor(c, config)
	switch {
	case err != nil:
		return err
	case mon == nil:
		return nil // tsmon is disabled
	}

	// Monitoring is enabled, so get the expensive default values for hostname,
	// etc.
	if config.AutoGenHostname {
		fl.Target.AutoGenHostname = true
	}
	if config.Hostname != "" {
		if fl.Target.DeviceHostname == "" {
			fl.Target.DeviceHostname = config.Hostname
		}
		if fl.Target.TaskHostname == "" {
			fl.Target.TaskHostname = config.Hostname
		}
	}
	if config.Region != "" {
		if fl.Target.DeviceRegion == "" {
			fl.Target.DeviceRegion = config.Region
		}
		if fl.Target.TaskRegion == "" {
			fl.Target.TaskRegion = config.Region
		}
	}
	fl.Target.SetDefaultsFromHostname()
	t, err := target.NewFromFlags(&fl.Target)
	if err != nil {
		return err
	}

	Initialize(c, mon, store.NewInMemory(t))

	state := GetState(c)
	if state.Flusher != nil {
		logging.Infof(c, "Canceling previous tsmon auto flush")
		state.Flusher.stop()
		state.Flusher = nil
	}

	if fl.Flush == FlushAuto {
		state.Flusher = &autoFlusher{}
		state.Flusher.start(c, fl.FlushInterval)
	}

	return nil
}

// Initialize configures the tsmon library with the given monitor and store.
func Initialize(c context.Context, m monitor.Monitor, s store.Store) {
	state := GetState(c)
	state.M = m
	state.SetStore(s)
}

// Shutdown gracefully terminates the tsmon by doing the final flush and
// disabling auto flush (if it was enabled).
//
// It resets Monitor and Store.
//
// Logs error to standard logger. Does nothing if tsmon wasn't initialized.
func Shutdown(c context.Context) {
	state := GetState(c)
	if store.IsNilStore(state.S) {
		return
	}

	if state.Flusher != nil {
		logging.Debugf(c, "Stopping tsmon auto flush")
		state.Flusher.stop()
		state.Flusher = nil
	}

	// Flush logs errors inside.
	Flush(c)

	// Reset the state as if 'InitializeFromFlags' was never called.
	Initialize(c, monitor.NewNilMonitor(), store.NewNilStore())
}

// ResetCumulativeMetrics resets only cumulative metrics.
func ResetCumulativeMetrics(c context.Context) {
	GetState(c).ResetCumulativeMetrics(c)
}

// initMonitor examines flags and config and initializes a monitor.
//
// It returns (nil, nil) if tsmon should be disabled.
func initMonitor(c context.Context, config config) (monitor.Monitor, error) {
	if config.Endpoint == "" {
		logging.Infof(c, "tsmon is disabled because no endpoint is configured")
		return nil, nil
	}
	if strings.ToLower(config.Endpoint) == "none" {
		logging.Infof(c, "tsmon is explicitly disabled")
		return nil, nil
	}

	endpointURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, err
	}

	switch endpointURL.Scheme {
	case "file":
		return monitor.NewDebugMonitor(endpointURL.Path), nil
	case "pubsub":
		client, err := clientFactory(c, config.Credentials)
		if err != nil {
			return nil, err
		}

		return monitor.NewPubsubMonitor(c, client, gcps.NewTopic(endpointURL.Host, strings.TrimPrefix(endpointURL.Path, "/")))
	default:
		return nil, fmt.Errorf("unknown tsmon endpoint url: %s", config.Endpoint)
	}
}

// makeClient returns http.Client that knows how to send authenticated requests
// to PubSub API.
func clientFactory(ctx context.Context, credentials string) (*http.Client, error) {
	authOpts := auth.Options{
		Scopes: gcps.PublisherScopes,
	}
	switch credentials {
	case GCECredentials:
		authOpts.Method = auth.GCEMetadataMethod

	case "":
		// Let the Authenticator choose.
		break

	default:
		authOpts.Method = auth.ServiceAccountMethod
		authOpts.ServiceAccountJSONPath = credentials
	}
	return auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
}
