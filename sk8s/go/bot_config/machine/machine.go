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

type Machine struct {
	store *store.StoreImpl

	adb adb.Adb

	machineID string

	interrogateTimer           metrics2.Float64SummaryMetric
	interrogateAndSendFailures metrics2.Counter
	storeWatchArrivalCounter   metrics2.Counter

	sink sink.Sink

	// mutex protects dims.
	mutex sync.Mutex

	// dimensions are the dimensions the machine state server wants us to report
	// to swarming.
	dimensions map[string][]string
}

func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*Machine, error) {
	store, err := store.New(ctx, false, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to build store instance.")
	}
	sink, err := sink.New(ctx, local, instanceConfig)
	return &Machine{
		dimensions:                 map[string][]string{},
		store:                      store,
		sink:                       sink,
		interrogateTimer:           metrics2.GetFloat64SummaryMetric("bot_config_machine_interrogate_timer"),
		interrogateAndSendFailures: metrics2.GetCounter("bot_config_machine_interrogate_and_send_errors"),
	}, nil
}

func (m *Machine) interrogateAndSend(ctx context.Context) error {
	event, err := m.Interrogate(ctx)
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

	// Start a loop that scans for local devices and sends pubsub events with all the data every 30s.
	if err := m.interrogateAndSend(ctx); err != nil {
		return skerr.Wrap(err)
	}

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
			m.mutex.Lock()
			m.dimensions = desc.Dimensions
			m.mutex.Unlock()
		}
	}()
	return nil
}

func (m *Machine) Interrogate(ctx context.Context) (machine.Event, error) {
	defer timer.NewWithSummary("interrogate", m.interrogateTimer).Stop()
	ret := machine.NewEvent()

	name := os.Getenv(swarming.SwarmingBotIDEnvVar)
	if name == "" {
		var err error
		name, err = os.Hostname()
		if err != nil {
			name = "unknown-machine"
		}
	}
	if m.machineID == "" {
		m.machineID = name
	}
	ret.Host.Name = name
	ret.Host.Rack = os.Getenv("MY_RACK_NAME")

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

func (m *Machine) Dims() map[string][]string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.dimensions
}
