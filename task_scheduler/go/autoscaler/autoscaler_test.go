package autoscaler

import (
	"math"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming/autoscaler"
	"go.skia.org/infra/go/testutils"
)

func TestAutoscaler(t *testing.T) {
	testutils.SmallTest(t)

	// Setup.
	s1, urlMock, instances := autoscaler.Setup(t)
	now := time.Now()
	as := New([]*autoscaler.Autoscaler{s1}, now)
	getCandidates := func(n int) [][]string {
		botDims := s1.Dimensions().Keys()
		rv := make([][]string, 0, n)
		for i := 0; i < n; i++ {
			rv = append(rv, botDims)
		}
		return rv
	}

	// We have len(instances) bots online. With no candidates we should shut
	// down half of the bots at every HALF_LIFE_HOUR.
	autoscaler.MockAllOnline(t, urlMock, instances)
	assert.NoError(t, as.Update())

	// Keep shutting down half of our bots until there are none left.
	candidates := getCandidates(0)
	lastStopped := len(instances)
	step := 0
	for lastStopped > MIN_BOTS {
		now = now.Add(HALF_LIFE_HOURS * time.Hour)
		stoppingN := int(math.Ceil(float64(lastStopped) / 2.0))
		stopRangeStart := lastStopped - stoppingN
		if stopRangeStart < MIN_BOTS {
			stopRangeStart = MIN_BOTS
		}
		stopping := instances[stopRangeStart:lastStopped]
		sklog.Infof("Stopping %d:%d", stopRangeStart, lastStopped)
		for _, instance := range stopping {
			autoscaler.MockStop(t, urlMock, instance.Name)
		}
		assert.NoError(t, as.autoscale(candidates, now))
		autoscaler.WaitForStop(urlMock)
		lastStopped -= stoppingN
		if lastStopped < MIN_BOTS {
			lastStopped = MIN_BOTS
		}
		assert.Equal(t, lastStopped, s1.NumOnline())
		step++
	}
	assert.Equal(t, 5, step)
	assert.Equal(t, MIN_BOTS, s1.NumOnline())
	assert.Equal(t, 0, s1.NumOffline()) // Because we never ran Update()
	assert.Equal(t, len(instances)-MIN_BOTS, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())

	// The bots have fully shut down. MIN_BOTS doesn't apply here.
	autoscaler.MockAllOffline(t, urlMock, instances)
	assert.NoError(t, as.Update())
	assert.Equal(t, 0, s1.NumOnline())
	assert.Equal(t, len(instances), s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())

	// Now we have some task candidates.
	startN := 5
	candidates = getCandidates(startN)
	starting := instances[:startN]
	for _, instance := range starting {
		autoscaler.MockStart(t, urlMock, instance.Name)
	}
	assert.NoError(t, as.autoscale(candidates, now))
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, s1.NumOnline())
	assert.Equal(t, len(instances)-startN, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, startN, s1.NumStarting())

	// Our instances are starting; ensure that we don't start any more on
	// the next call, since they should come up soon.
	assert.NoError(t, as.autoscale(candidates, now))
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, s1.NumOnline())
	assert.Equal(t, len(instances)-startN, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, startN, s1.NumStarting())

	// Now they're online, but they're busy and we have more candidates.
	for idx, instance := range instances {
		if idx < startN {
			autoscaler.MockOnlineAndBusy(t, urlMock, instance.Name)
		} else {
			autoscaler.MockOffline(t, urlMock, instance.Name)
		}
	}
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, startN, s1.NumOnline())
	assert.Equal(t, len(instances)-startN, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())
	assert.Equal(t, startN, s1.NumBusy())
	assert.Equal(t, 0, s1.NumIdle())
	starting = instances[startN : 2*startN]
	for _, instance := range starting {
		autoscaler.MockStart(t, urlMock, instance.Name)
	}
	assert.NoError(t, as.autoscale(candidates, now))
	assert.True(t, urlMock.Empty())
	assert.Equal(t, startN, s1.NumOnline())
	assert.Equal(t, len(instances)-2*startN, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, startN, s1.NumStarting())

	// Force all bots online. We have candidates, but still have more bots
	// than we need. Ensure that we stop instances, but leave enough running
	// to run all of our candidates.
	now = now.Add(HALF_LIFE_HOURS * time.Hour)
	autoscaler.MockAllOnline(t, urlMock, instances)
	assert.NoError(t, as.Update())
	assert.Equal(t, len(instances), s1.NumOnline())
	assert.Equal(t, 0, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())
	numCandidates := 67
	stopping := instances[numCandidates:]
	for _, instance := range stopping {
		autoscaler.MockStop(t, urlMock, instance.Name)
	}
	candidates = getCandidates(numCandidates)
	assert.NoError(t, as.autoscale(candidates, now))
	autoscaler.WaitForStop(urlMock)
	assert.Equal(t, numCandidates, s1.NumOnline())
	assert.Equal(t, 0, s1.NumOffline())
	assert.Equal(t, len(instances)-numCandidates, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())

	// Now all of those bots are offline.
	for idx, instance := range instances {
		if idx < numCandidates {
			autoscaler.MockOnline(t, urlMock, instance.Name)
		} else {
			autoscaler.MockOffline(t, urlMock, instance.Name)
		}
	}
	assert.NoError(t, as.Update())
	assert.Equal(t, len(instances)-len(stopping), s1.NumOnline())
	assert.Equal(t, len(stopping), s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())

	// We have more candidates than the maximum number of bots.
	autoscaler.MockAllOnline(t, urlMock, instances)
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, len(instances), s1.NumOnline())
	assert.Equal(t, 0, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())
	candidates = getCandidates(len(instances) * 2)
	assert.NoError(t, as.autoscale(candidates, now))
	assert.True(t, urlMock.Empty())
	assert.Equal(t, len(instances), s1.NumOnline())
	assert.Equal(t, 0, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())

	// Some bots are online, some bots are stopping. We have more candidates
	// than we can run, so we should start some bots.
	now = now.Add(10 * time.Hour)
	for idx, instance := range instances {
		if idx < 75 {
			autoscaler.MockOnline(t, urlMock, instance.Name)
		} else {
			autoscaler.MockOffline(t, urlMock, instance.Name)
		}
	}
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 75, s1.NumOnline())
	assert.Equal(t, len(instances)-75, s1.NumOffline())
	assert.Equal(t, 0, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())
	assert.Equal(t, 0, s1.NumBusy())
	assert.Equal(t, 75, s1.NumIdle())
	for _, instance := range instances[50:75] {
		autoscaler.MockStop(t, urlMock, instance.Name)
	}
	candidates = getCandidates(50)
	assert.NoError(t, as.autoscale(candidates, now))
	autoscaler.WaitForStop(urlMock)
	assert.Equal(t, 50, s1.NumOnline())
	assert.Equal(t, len(instances)-75, s1.NumOffline())
	assert.Equal(t, 25, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())
	for idx, instance := range instances {
		if idx < 75 {
			autoscaler.MockOnline(t, urlMock, instance.Name)
		} else {
			autoscaler.MockOffline(t, urlMock, instance.Name)
		}
	}
	assert.NoError(t, as.Update())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 50, s1.NumOnline())
	assert.Equal(t, len(instances)-75, s1.NumOffline())
	assert.Equal(t, 25, s1.NumStopping())
	assert.Equal(t, 0, s1.NumStarting())
	assert.Equal(t, 0, s1.NumBusy())
	assert.Equal(t, 50, s1.NumIdle())
	for _, instance := range instances[75:] {
		autoscaler.MockStart(t, urlMock, instance.Name)
	}
	candidates = getCandidates(100)
	assert.NoError(t, as.autoscale(candidates, now))
	autoscaler.WaitForStop(urlMock)
	assert.Equal(t, 50, s1.NumOnline())
	assert.Equal(t, 0, s1.NumOffline())
	assert.Equal(t, 25, s1.NumStopping())
	assert.Equal(t, 25, s1.NumStarting())
	assert.Equal(t, 0, s1.NumBusy())
}
