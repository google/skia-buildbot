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

// Machine is the interface to the machine state server. Seee //machine.
type Machine struct {
	// store is the firestore back store for machine state.
	store *store.StoreImpl

	// sink is how we send machine.Events to the the machine state server.
	sink sink.Sink

	// adb makes calls to the adb server.
	adb adb.Adb

	// machineID is the swarming id of the machine.
	machineID string

	// rack is the physical rack name, e.g. rack4.
	rack string

	// Metrics
	interrogateTimer           metrics2.Float64SummaryMetric
	interrogateAndSendFailures metrics2.Counter
	storeWatchArrivalCounter   metrics2.Counter

	// mutex protects dimsensions
	mutex sync.Mutex

	// dimensions are the dimensions the machine state server wants us to report
	// to swarming.
	dimensions map[string][]string
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
	if machineID == "" {
		var err error
		machineID, err = os.Hostname()
		if err != nil {
			machineID = "unknown-machine"
		}
	}

	return &Machine{
		dimensions:                 machine.SwarmingDimensions{},
		store:                      store,
		sink:                       sink,
		machineID:                  machineID,
		rack:                       os.Getenv("MY_RACK_NAME"),
		interrogateTimer:           metrics2.GetFloat64SummaryMetric("bot_config_machine_interrogate_timer"),
		interrogateAndSendFailures: metrics2.GetCounter("bot_config_machine_interrogate_and_send_errors"),
		storeWatchArrivalCounter:   metrics2.GetCounter("bot_config_machine_store_watch_arrival"),
	}, nil
}

// interrogate the machine we are running on and return all that info in a machine.Event.
func (m *Machine) interrogate(ctx context.Context) (machine.Event, error) {
	defer timer.NewWithSummary("interrogate", m.interrogateTimer).Stop()

	ret := machine.NewEvent()
	ret.Host.Name = m.machineID
	ret.Host.Rack = m.rack

	props, err := m.adb.RawProperties(ctx)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to read android properties.")
	}
	ret.Android.GetProp = props

	battery, err := m.adb.RawDumpSys(ctx, "battery")
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to read android battery status.")
	}
	ret.Android.DumpsysBattery = battery

	thermal, err := m.adb.RawDumpSys(ctx, "thermalservice")
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to read android thermal status.")
	}
	ret.Android.DumpsysThermalService = thermal

	return ret, nil
}

func (m *Machine) interrogateAndSend(ctx context.Context) error {
	event, err := m.interrogate(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed first interrogation step.")
	}
	err = m.sink.Send(ctx, event)
	if err != nil {
		return skerr.Wrapf(err, "Failed to send interrogation step.")
	}
	return nil
}

func (m *Machine) Start(ctx context.Context) error {
	if err := m.interrogateAndSend(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Start a loop that scans for local devices and sends pubsub events with all the data every 30s.
	go util.RepeatCtx(ctx, 30*time.Second, func(ctx context.Context) {
		if err := m.interrogateAndSend(ctx); err != nil {
			m.interrogateAndSendFailures.Inc(1)
			sklog.Errorf("interrogateAndSend failed: %s", err)
		}
	})

	// Also start a second loop that does a firestore onsnapshot watcher that gets the dims we should
	// be reporting to swarming.
	go func() {
		for desc := range m.store.Watch(ctx, m.machineID) {
			m.storeWatchArrivalCounter.Inc(1)
			m.mutex.Lock()
			m.dimensions = desc.Dimensions
			m.mutex.Unlock()
		}
	}()
	return nil
}

func (m *Machine) Dims() machine.SwarmingDimensions {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.dimensions
}
