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
	"go.skia.org/infra/machine/go/test_machine_monitor/swarming"
	"google.golang.org/api/option"
)

const (
	adbShellGetPropSuccess = `[ro.product.manufacturer]: [asus]`
	adbShellDumpSysBattery = `This is dumpsys output.`
	versionForTest         = "some-version-string-for-testing-purposes"
)

func setupConfig(t *testing.T) (context.Context, *pubsub.Topic, config.InstanceConfig) {
	unittest.RequiresFirestoreEmulator(t)
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
		require.NoError(t, err)
	}
	topic.Stop()
	assert.NoError(t, err)

	return ctx, topic, instanceConfig
}

func TestStart_InterrogatesDeviceInitiallyAndOnTimer(t *testing.T) {
	// Manual because we are testing pubsub.
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)
	ctx, cancel := context.WithCancel(ctx)

	// Use source to read pubsub events.
	source, err := pubsubsource.New(ctx, true, instanceConfig)
	require.NoError(t, err)

	// Set the SWARMING_BOT_ID env variable.
	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	err = os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
		require.NoError(t, err)
	}()

	const imageName = "gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean"

	// Set the IMAGE_NAME env variable.
	oldImageVar := os.Getenv(swarming.KubernetesImageEnvVar)
	err = os.Setenv(swarming.KubernetesImageEnvVar, imageName)
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.KubernetesImageEnvVar, oldImageVar)
		require.NoError(t, err)
	}()

	// Create a Machine instance.
	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	m, err := New(ctx, true, instanceConfig, start, versionForTest, true)
	require.NoError(t, err)
	assert.Equal(t, "my-test-bot-001", m.MachineID)

	// Write a description into firestore. We expect the dimensions here to
	// bubble down to the machine
	err = m.store.Update(ctx, "my-test-bot-001", func(machine.Description) machine.Description {
		ret := machine.NewDescription(ctx)
		ret.Mode = machine.ModeMaintenance
		ret.Dimensions["foo"] = []string{"bar"}
		return ret
	})
	require.NoError(t, err)

	// Set up fakes for adb. We have two sets of 3 since Start calls
	// interrogateAndSend, and then util.RepeatCtx, which also calls
	// interrogateAndSend.
	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbShellGetProp_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_AdbShellGetProp_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_RawDumpSys_Success",
	)

	// Call Start().
	err = m.Start(ctx)
	require.NoError(t, err)

	// Start() emits a pubsub event before it returns, so check we received the
	// expected machine.Event.
	ch, err := source.Start(ctx)
	require.NoError(t, err)
	event := <-ch

	hostname, err := os.Hostname()
	require.NoError(t, err)

	assert.Equal(t,
		machine.Event{
			EventType: "raw_state",
			Android: machine.Android{
				GetProp:               adbShellGetPropSuccess,
				DumpsysBattery:        adbShellDumpSysBattery,
				DumpsysThermalService: adbShellDumpSysBattery,
			},
			Host: machine.Host{
				Name:            "my-test-bot-001",
				PodName:         hostname,
				KubernetesImage: imageName,
				StartTime:       start,
			},
		},
		event)

	// Let the machine.Event get sent via pubsub. OK since this is a manual test.
	time.Sleep(time.Second)

	// Cancel both Go routines inside Start().
	cancel()

	// Confirm the context is cancelled by waiting for the channel to close.
	for range ch {
	}

	assert.Equal(t, int64(1), m.storeWatchArrivalCounter.Get())
	assert.Equal(t, int64(0), m.interrogateAndSendFailures.Get())

	// Confirm the firestore write made it all the way to Dims().
	assert.Equal(t, machine.SwarmingDimensions{"foo": {"bar"}}, m.DimensionsForSwarming())
}

func TestStart_FirestoreWritesGetReflectedToMachine(t *testing.T) {
	// Manual because we are testing pubsub.
	unittest.ManualTest(t)

	ctx, _, instanceConfig := setupConfig(t)
	ctx, cancel := context.WithCancel(ctx)

	// Set the SWARMING_BOT_ID env variable.
	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	err := os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
		require.NoError(t, err)
	}()

	const imageName = "gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean"

	// Set the IMAGE_NAME env variable.
	oldImageVar := os.Getenv(swarming.KubernetesImageEnvVar)
	err = os.Setenv(swarming.KubernetesImageEnvVar, imageName)
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.KubernetesImageEnvVar, oldImageVar)
		require.NoError(t, err)
	}()

	// Create a Machine instance.
	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	m, err := New(ctx, true, instanceConfig, start, versionForTest, true)
	require.NoError(t, err)
	assert.Equal(t, "my-test-bot-001", m.MachineID)

	assert.False(t, m.GetMaintenanceMode())

	// Start just the Firestore watcher.
	m.startStoreWatch(ctx)

	// Write a description into firestore. We expect the dimensions and mode to
	// bubble down to the machine
	err = m.store.Update(ctx, "my-test-bot-001", func(machine.Description) machine.Description {
		ret := machine.NewDescription(ctx)
		ret.Mode = machine.ModeMaintenance
		ret.Dimensions["foo"] = []string{"bar"}
		return ret
	})
	require.NoError(t, err)

	assert.Equal(t, machine.SwarmingDimensions{"foo": {"bar"}}, m.DimensionsForSwarming())
	assert.True(t, m.GetMaintenanceMode())

	// Now change the mode.
	err = m.store.Update(ctx, "my-test-bot-001", func(machine.Description) machine.Description {
		ret := machine.NewDescription(ctx)
		ret.Mode = machine.ModeAvailable
		return ret
	})
	require.NoError(t, err)

	// Confirm we go out of maintenance mode.
	assert.False(t, m.GetMaintenanceMode())

	// Cancel Go routine inside startStoreWatch.
	cancel()

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

func TestStart_AdbFailsToTalkToDevice_EmptyEventsSentToServer(t *testing.T) {
	// Manual because we are testing pubsub.
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)
	ctx, cancel := context.WithCancel(ctx)

	// Use source to read pubsub events.
	source, err := pubsubsource.New(ctx, true, instanceConfig)
	require.NoError(t, err)

	// Set the SWARMING_BOT_ID env variable.
	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	err = os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
		assert.NoError(t, err)
	}()

	// Create a Machine instance.
	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	m, err := New(ctx, true, instanceConfig, start, versionForTest, true)
	require.NoError(t, err)

	// Set up fakes for adb. We have two sets of 3 since Start calls
	// interrogateAndSend, and then util.RepeatCtx, which also calls
	// interrogateAndSend.
	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
	)

	// Call Start().
	err = m.Start(ctx)
	require.NoError(t, err)

	// Start() emits a pubsub event before it returns, so check we received the
	// expected machine.Event.
	ch, err := source.Start(ctx)
	require.NoError(t, err)
	event := <-ch

	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t,
		machine.Event{
			EventType: "raw_state",
			Android: machine.Android{
				GetProp:               "",
				DumpsysBattery:        "",
				DumpsysThermalService: "",
			},
			Host: machine.Host{
				Name:      "my-test-bot-001",
				PodName:   hostname,
				StartTime: start,
			},
		},
		event)

	// Let the machine.Event get sent via pubsub. OK since this is a manual test.
	time.Sleep(time.Second)

	// Cancel both Go routines inside Start().
	cancel()

	// Confirm the context is cancelled by waiting for the channel to close.
	for range ch {
	}

	assert.Equal(t, int64(0), m.interrogateAndSendFailures.Get())
}

func Test_FakeExe_AdbFail(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	os.Exit(1)
}

func TestStart_RunningSwarmingTaskInMachineIsSentInEvent(t *testing.T) {
	// Manual because we are testing pubsub.
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)
	ctx, cancel := context.WithCancel(ctx)

	// Use source to read pubsub events.
	source, err := pubsubsource.New(ctx, true, instanceConfig)
	require.NoError(t, err)

	// Set the SWARMING_BOT_ID env variable.
	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	err = os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
		assert.NoError(t, err)
	}()

	// Create a Machine instance.
	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	m, err := New(ctx, true, instanceConfig, start, versionForTest, true)
	require.NoError(t, err)
	// We are running a task.
	m.runningTask = true
	require.NoError(t, err)

	// Set up fakes for adb. We have two sets of 3 since Start calls
	// interrogateAndSend, and then util.RepeatCtx, which also calls
	// interrogateAndSend.
	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
	)

	// Call Start().
	err = m.Start(ctx)
	require.NoError(t, err)

	// Start() emits a pubsub event before it returns, so check we received the
	// expected machine.Event.
	ch, err := source.Start(ctx)
	require.NoError(t, err)
	event := <-ch

	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t,
		machine.Event{
			EventType: "raw_state",
			Android: machine.Android{
				GetProp:               "",
				DumpsysBattery:        "",
				DumpsysThermalService: "",
			},
			Host: machine.Host{
				Name:      "my-test-bot-001",
				PodName:   hostname,
				StartTime: start,
			},
			RunningSwarmingTask: true,
			LaunchedSwarming:    true,
		},
		event)

	// Let the machine.Event get sent via pubsub. OK since this is a manual test.
	time.Sleep(time.Second)

	// Cancel both Go routines inside Start().
	cancel()

	// Confirm the context is cancelled by waiting for the channel to close.
	for range ch {
	}

	assert.Equal(t, int64(0), m.interrogateAndSendFailures.Get())
}

func TestRebootDevice_Success(t *testing.T) {
	// Manual because we are testing pubsub.
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)

	// Set the SWARMING_BOT_ID env variable.
	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	err := os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
		assert.NoError(t, err)
	}()

	// Create a Machine instance.
	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	m, err := New(ctx, true, instanceConfig, start, versionForTest, true)
	require.NoError(t, err)
	m.UpdateDescription(machine.Description{
		Dimensions: machine.SwarmingDimensions{
			machine.DimAndroidDevices: {"1"},
		},
	})

	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbReboot_Success",
	)

	err = m.RebootDevice(ctx)
	require.NoError(t, err)
}

func Test_FakeExe_AdbReboot_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "reboot"}, args)

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func TestRebootDevice_ErrOnNonZeroExitCode(t *testing.T) {
	// Manual because we are testing pubsub.
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)

	// Set the SWARMING_BOT_ID env variable.
	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	err := os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
		assert.NoError(t, err)
	}()

	// Create a Machine instance.
	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	m, err := New(ctx, true, instanceConfig, start, versionForTest, true)
	require.NoError(t, err)
	m.UpdateDescription(machine.Description{
		Dimensions: machine.SwarmingDimensions{
			machine.DimAndroidDevices: {"1"},
		},
	})

	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_Reboot_NonZeroExitCode",
	)

	err = m.RebootDevice(ctx)
	require.Error(t, err)
}

func TestRebootDevice_NoErrorIfNoAndroidDeviceAttached(t *testing.T) {
	// Manual because we are testing pubsub.
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)

	// Set the SWARMING_BOT_ID env variable.
	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	err := os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	require.NoError(t, err)
	defer func() {
		err = os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
		assert.NoError(t, err)
	}()

	// Create a Machine instance.
	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	m, err := New(ctx, true, instanceConfig, start, versionForTest, true)
	require.NoError(t, err)

	m.UpdateDescription(machine.Description{
		Dimensions: machine.SwarmingDimensions{},
	})

	// If adb reboot gets called it will return an error.
	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_Reboot_NonZeroExitCode",
	)

	err = m.RebootDevice(ctx)

	// Since the dimensions say there's no Android device attached we shouldn't run
	// "adb reboot" and should not return an error.
	require.NoError(t, err)
}

func Test_FakeExe_Reboot_NonZeroExitCode(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	_, _ = fmt.Fprintf(os.Stderr, "error: no devices/emulators found")

	os.Exit(127)
}
