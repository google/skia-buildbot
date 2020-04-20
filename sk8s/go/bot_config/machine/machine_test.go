// Package machine is for interacting with the machine state server. See //machine.
package machine

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/source/pubsubsource"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/sk8s/go/bot_config/swarming"
	"google.golang.org/api/option"
)

const (
	adbShellGetPropSuccess = `[ro.product.manufacturer]: [asus]`
	adbShellDumpSysBattery = `This is dumpsys output.`
)

func setupConfig(t *testing.T) (context.Context, *pubsub.Topic, config.InstanceConfig) {
	require.NotEmpty(t, os.Getenv("FIRESTORE_EMULATOR_HOST"), "This test requires the firestore emulator.")
	unittest.RequiresPubSubEmulator(t)

	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	instanceConfig := config.InstanceConfig{
		Source: config.Source{
			Project: "test-project",
			Topic:   fmt.Sprintf("events-%d", rand.Int63()),
		},
		Store: config.Store{
			Project:  "test-project",
			Instance: fmt.Sprintf("test-%d", rand.Int63()),
		},
	}

	ts, err := auth.NewDefaultTokenSource(true, pubsub.ScopePubSub)
	require.NoError(t, err)
	pubsubClient, err := pubsub.NewClient(ctx, instanceConfig.Source.Project, option.WithTokenSource(ts))
	require.NoError(t, err)

	// Create the topic.
	topic := pubsubClient.Topic(instanceConfig.Source.Topic)
	ok, err := topic.Exists(ctx)
	require.NoError(t, err)
	if !ok {
		topic, err = pubsubClient.CreateTopic(ctx, instanceConfig.Source.Topic)
	}
	topic.Stop()
	assert.NoError(t, err)

	return ctx, topic, instanceConfig
}

func TestStart(t *testing.T) {
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)
	ctx, cancel := context.WithCancel(ctx)

	source, err := pubsubsource.New(ctx, true, instanceConfig)
	require.NoError(t, err)

	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	defer func() {
		os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
	}()

	m, err := New(ctx, true, instanceConfig)
	require.NoError(t, err)

	assert.Equal(t, "my-test-bot-001", m.machineID)

	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbShellGetProp_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_AdbShellGetProp_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_RawDumpSys_Success",
	)

	err = m.store.Update(ctx, "my-test-bot-001", func(machine.Description) machine.Description {
		ret := machine.NewDescription()
		ret.Mode = machine.ModeMaintenance
		ret.Dimensions["foo"] = []string{"bar"}
		return ret
	})
	require.NoError(t, err)

	m.Start(ctx)
	assert.Equal(t, 3, executil.FakeCommandsReturned(ctx))

	ch, err := source.Start(ctx)
	require.NoError(t, err)
	event := <-ch

	assert.Equal(t,
		machine.Event{
			EventType: "raw_state",
			Android: machine.Android{
				GetProp:               "[ro.product.manufacturer]: [asus]",
				DumpsysBattery:        "This is dumpsys output.",
				DumpsysThermalService: "This is dumpsys output.",
			},
			Host: machine.Host{
				Name: "my-test-bot-001",
				Rack: "",
			},
		},
		event)

	cancel()
	for range ch {
	}

	assert.Equal(t, int64(1), m.storeWatchArrivalCounter.Get())
	assert.Equal(t, machine.SwarmingDimensions{"foo": {"bar"}}, m.Dims())
}

func Test_FakeExe_AdbShellGetProp_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "getprop"}, args)

	fmt.Print(adbShellGetPropSuccess)
	os.Exit(0)
}

func Test_FakeExe_RawDumpSys_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	fmt.Print(adbShellDumpSysBattery)
	os.Exit(0)
}
