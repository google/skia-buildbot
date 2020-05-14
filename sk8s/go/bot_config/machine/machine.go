// Package machine is for interacting with the machine state server. See //machine.
package machine

import (
	"context"
	"os"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/sink"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/sk8s/go/bot_config/adb"
	"go.skia.org/infra/sk8s/go/bot_config/swarming"
)

const (
	interrogateDuration = 30 * time.Second
)

// Machine is the interface to the machine state server. See //machine.
type Machine struct {
	// store is the firestore backend store for machine state.
	store *store.StoreImpl

	// sink is how we send machine.Events to the the machine state server.
	sink sink.Sink

	// adb makes calls to the adb server.
	adb adb.Adb

	// MachineID is the swarming id of the machine.
	MachineID string

	// Hostname is the hostname(), which is the pod name under k8s.
	Hostname string

	// KubernetesImage is the container image being run.
	KubernetesImage string

	// Metrics
	interrogateTimer           metrics2.Float64SummaryMetric
	interrogateAndSendFailures metrics2.Counter
	storeWatchArrivalCounter   metrics2.Counter

	// mutex protects dimensions and runningTask.
	mutex sync.Mutex

	// dimensions are the dimensions the machine state server wants us to report
	// to swarming.
	dimensions machine.SwarmingDimensions

	// runningTask is true if the machine is currently running a swarming task.
	runningTask bool
}

// New return an instance of *Machine.
func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*Machine, error) {
	store, err := store.New(ctx, false, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to build store instance.")
	}
	sink, err := sink.New(ctx, local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to build sink instance.")
	}

	machineID := os.Getenv(swarming.SwarmingBotIDEnvVar)
	kubernetesImage := os.Getenv(swarming.KubernetesImageEnvVar)
	hostname, err := os.Hostname()
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not determine hostname.")
	}

	return &Machine{
		dimensions:                 machine.SwarmingDimensions{},
		store:                      store,
		sink:                       sink,
		adb:                        adb.New(),
		MachineID:                  machineID,
		Hostname:                   hostname,
		KubernetesImage:            kubernetesImage,
		interrogateTimer:           metrics2.GetFloat64SummaryMetric("bot_config_machine_interrogate_timer", map[string]string{"machine": machineID}),
		interrogateAndSendFailures: metrics2.GetCounter("bot_config_machine_interrogate_and_send_errors", map[string]string{"machine": machineID}),
		storeWatchArrivalCounter:   metrics2.GetCounter("bot_config_machine_store_watch_arrival", map[string]string{"machine": machineID}),
	}, nil
}

// interrogate the machine we are running on and return all that info in a machine.Event.
func (m *Machine) interrogate(ctx context.Context) machine.Event {
	defer timer.NewWithSummary("interrogate", m.interrogateTimer).Stop()

	ret := machine.NewEvent()
	ret.Host.Name = m.MachineID
	ret.Host.PodName = m.Hostname
	ret.Host.KubernetesImage = m.KubernetesImage

	if props, err := m.adb.RawProperties(ctx); err != nil {
		sklog.Infof("Failed to read android properties: %s", err)
	} else {
		ret.Android.GetProp = props
	}

	if battery, err := m.adb.RawDumpSys(ctx, "battery"); err != nil {
		sklog.Infof("Failed to read android battery status: %s", err)
	} else {
		ret.Android.DumpsysBattery = battery
	}

	if thermal, err := m.adb.RawDumpSys(ctx, "thermalservice"); err != nil {
		sklog.Infof("Failed to read android thermal status: %s", err)
	} else {
		ret.Android.DumpsysThermalService = thermal
	}

	ret.RunningSwarmingTask = m.runningTask

	return ret
}

func (m *Machine) interrogateAndSend(ctx context.Context) error {
	event := m.interrogate(ctx)
	if err := m.sink.Send(ctx, event); err != nil {
		return skerr.Wrapf(err, "Failed to send interrogation step.")
	}
	return nil
}

// Start the background processes that send events to the sink and watch for
// firestore changes.
func (m *Machine) Start(ctx context.Context) error {
	if err := m.interrogateAndSend(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Start a loop that scans for local devices and sends pubsub events with all the data every 30s.
	go util.RepeatCtx(ctx, interrogateDuration, func(ctx context.Context) {
		if err := m.interrogateAndSend(ctx); err != nil {
			m.interrogateAndSendFailures.Inc(1)
			sklog.Errorf("interrogateAndSend failed: %s", err)
		}
	})

	// Also start a second loop that does a firestore onsnapshot watcher that gets the dims we should
	// be reporting to swarming.
	go func() {
		for desc := range m.store.Watch(ctx, m.MachineID) {
			m.storeWatchArrivalCounter.Inc(1)
			m.SetDimensionsForSwarming(desc.Dimensions)
		}
	}()
	return nil
}

// SetDimensionsForSwarming sets the dimensions that should be reported to swarming. Should only
// be called by tests.
func (m *Machine) SetDimensionsForSwarming(dims machine.SwarmingDimensions) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.dimensions = dims
}

// DimensionsForSwarming returns the dimensions that should be reported to swarming.
func (m *Machine) DimensionsForSwarming() machine.SwarmingDimensions {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.dimensions
}

// SetIsRunningSwarmingTask records if a swarming task is being run.
func (m *Machine) SetIsRunningSwarmingTask(isRunning bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.runningTask = isRunning
}

// IsRunningSwarmingTask returns true is a swarming task is currently running.
func (m *Machine) IsRunningSwarmingTask() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.runningTask
}
