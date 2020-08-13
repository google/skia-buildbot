package autoscaler

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAutoscaler(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	as, urlMock, instances := Setup(t)

	// Initial autoscaler creation.
	assert.Equal(t, 100, as.Total())
	assert.Equal(t, 100, as.NumOnline())
	assert.Equal(t, 0, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())
	deepequal.AssertDeepEqual(t, dims, as.Dimensions())

	MockAllOffline(t, urlMock, instances)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, as.NumOnline())
	assert.Equal(t, 100, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())

	// Start an instance.
	vm1 := instances[0]
	MockStart(t, urlMock, vm1.Name)
	assert.NoError(t, as.Start([]string{vm1.Name}))
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, as.NumOnline())
	assert.Equal(t, 99, as.NumOffline())
	assert.Equal(t, 1, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())
	assert.EqualError(t, as.Start([]string{vm1.Name}), "Bot skia-e-gce-001 cannot be started because it is in \"STARTING\" state.")
	assert.EqualError(t, as.Stop([]string{vm1.Name}), "Bot skia-e-gce-001 cannot be stopped because it is in \"STARTING\" state.")

	// Update, ensure we get the same result.
	MockAllOffline(t, urlMock, instances)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, as.NumOnline())
	assert.Equal(t, 99, as.NumOffline())
	assert.Equal(t, 1, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())

	// Now the instance is online.
	statuses := make(map[string]*FakeSwarmingStatus, len(instances))
	statuses[vm1.Name] = MockOnline(t, urlMock, vm1.Name)
	for _, instance := range instances[1:] {
		statuses[instance.Name] = MockOffline(t, urlMock, instance.Name)
	}
	MockSwarmingStatuses(t, urlMock, statuses)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 1, as.NumOnline())
	assert.Equal(t, 99, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())

	// Stop the instance.
	MockStop(t, urlMock, vm1.Name)
	assert.NoError(t, as.Stop([]string{vm1.Name}))
	assert.Equal(t, 0, as.NumOnline())
	assert.Equal(t, 99, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 1, as.NumStopping())
	assert.EqualError(t, as.Start([]string{vm1.Name}), "Bot skia-e-gce-001 cannot be started because it is in \"STOPPING\" state.")
	assert.EqualError(t, as.Stop([]string{vm1.Name}), "Bot skia-e-gce-001 cannot be stopped because it is in \"STOPPING\" state.")
	WaitForStop(urlMock)

	// Now the instance is offline.
	MockAllOffline(t, urlMock, instances)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, as.NumOnline())
	assert.Equal(t, 100, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())

	// Bring 10 instances online.
	n := 10
	for _, instance := range instances[:n] {
		MockStart(t, urlMock, instance.Name)
	}
	assert.NoError(t, as.StartN(n))
	assert.Equal(t, 0, as.NumOnline())
	assert.Equal(t, 90, as.NumOffline())
	assert.Equal(t, 10, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())
	statuses = make(map[string]*FakeSwarmingStatus, len(instances))
	for idx, instance := range instances {
		if idx < n {
			statuses[instance.Name] = MockOnline(t, urlMock, instance.Name)
		} else {
			statuses[instance.Name] = MockOffline(t, urlMock, instance.Name)
		}
	}
	MockSwarmingStatuses(t, urlMock, statuses)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 10, as.NumOnline())
	assert.Equal(t, 90, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())

	// Now, most of the bots are busy.
	statuses = make(map[string]*FakeSwarmingStatus, len(instances))
	for idx, instance := range instances {
		if idx < n {
			if (idx+1)%3 == 0 {
				statuses[instance.Name] = MockOnline(t, urlMock, instance.Name)
			} else {
				statuses[instance.Name] = MockOnlineAndBusy(t, urlMock, instance.Name)
			}
		} else {
			statuses[instance.Name] = MockOffline(t, urlMock, instance.Name)
		}
	}
	MockSwarmingStatuses(t, urlMock, statuses)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 10, as.NumOnline())
	assert.Equal(t, 90, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())
	assert.Equal(t, 7, as.NumBusy())
	assert.Equal(t, 3, as.NumIdle())

	// Stop some bots. Assert that we stop idle bots first, in reverse
	// alphanumeric order.
	MockStop(t, urlMock, instances[5].Name)
	MockStop(t, urlMock, instances[8].Name)
	assert.NoError(t, as.StopN(2))
	assert.Equal(t, 8, as.NumOnline())
	assert.Equal(t, 90, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 2, as.NumStopping())
	assert.Equal(t, 7, as.NumBusy())
	assert.Equal(t, 1, as.NumIdle())
	WaitForStop(urlMock)

	// Stop more bots. Now we'll issue termination requests of non-idle bots.
	MockStop(t, urlMock, instances[2].Name)
	MockStop(t, urlMock, instances[9].Name)
	MockStop(t, urlMock, instances[7].Name)
	MockStop(t, urlMock, instances[6].Name)
	assert.NoError(t, as.StopN(4))
	assert.Equal(t, 4, as.NumOnline())
	assert.Equal(t, 90, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 6, as.NumStopping())
	assert.Equal(t, 4, as.NumBusy())
	assert.Equal(t, 0, as.NumIdle())
	WaitForStop(urlMock)

	// Update, ensure we get the expected statuses.
	statuses = map[string]*FakeSwarmingStatus{}
	for idx, instance := range instances {
		if idx < n {
			MockOnlineAndBusy(t, urlMock, instance.Name)
			statuses[instance.Name] = &FakeSwarmingStatus{
				Online: true,
				Busy:   true,
			}
		} else {
			MockOffline(t, urlMock, instance.Name)
			statuses[instance.Name] = &FakeSwarmingStatus{
				Online: false,
				Busy:   false,
			}
		}
	}
	MockSwarmingStatuses(t, urlMock, statuses)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 4, as.NumOnline())
	assert.Equal(t, 90, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 6, as.NumStopping())
	assert.Equal(t, 4, as.NumBusy())
	assert.Equal(t, 0, as.NumIdle())

	// Test offline status.
	statuses = map[string]*FakeSwarmingStatus{}
	MockSwarmingStatuses(t, urlMock, statuses)
	for _, instance := range instances {
		mockGCEInstanceStatus(t, urlMock, instance.Name, "TERMINATED")
	}
	assert.NoError(t, as.Update())
	assert.Equal(t, 0, as.NumOnline())
	assert.Equal(t, 100, as.NumOffline())
	assert.Equal(t, 0, as.NumStarting())
	assert.Equal(t, 0, as.NumStopping())
}
