package processor

import (
	"errors"
	"io/ioutil"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestProcessor(t *testing.T) {
	testutils.MediumTest(t)

	// Setup.
	now := time.Now()
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Test function to verify that we called ProcessFn with the expected
	// time chunks.
	type invocation struct {
		start  time.Time
		end    time.Time
		retVal error
	}
	expect := []*invocation{}
	testProcess := func(start, end time.Time) error {
		e := expect[0]
		assert.Equal(t, e.start, start)
		if util.TimeIsZero(e.end) {
			// TODO
		} else {
			assert.Equal(t, e.end, end)
		}
		expect = expect[1:]
		return e.retVal
	}

	// Create the Processor.
	chunkSize := time.Hour
	p := &Processor{
		BeginningOfTime: now.Add(-24 * time.Hour),
		ChunkSize:       chunkSize,
		Name:            "test_processor",
		Frequency:       time.Millisecond,
		ProcessFn:       testProcess,
		Window:          3 * time.Hour,
		Workdir:         wd,
	}
	assert.NoError(t, p.init())

	// Verify the initial processing of data since the beginning of time.
	start := p.BeginningOfTime
	for start.Before(now) {
		end := start.Add(chunkSize)
		if now.Before(end) {
			end = now
		}
		expect = append(expect, &invocation{
			start: start,
			end:   end,
		})
		start = end
	}
	assert.NoError(t, p.run(now))
	assert.Empty(t, expect)
	assert.Equal(t, now, p.processedUpTo)

	// Verify the subsequent repeated processing of the window.
	start = now.Add(-p.Window)
	for start.Before(now) {
		end := start.Add(chunkSize)
		if now.Before(end) {
			end = now
		}
		expect = append(expect, &invocation{
			start: start,
			end:   end,
		})
		start = end
	}
	assert.NoError(t, p.run(now))
	assert.Empty(t, expect)
	assert.Equal(t, now, p.processedUpTo)

	// Another window, a little later now.
	now = now.Add(30 * time.Minute)
	start = p.processedUpTo.Add(-p.Window)
	for start.Before(now) {
		end := start.Add(chunkSize)
		if now.Before(end) {
			end = now
		}
		expect = append(expect, &invocation{
			start: start,
			end:   end,
		})
		start = end
	}
	assert.NoError(t, p.run(now))
	assert.Empty(t, expect)
	assert.Equal(t, now, p.processedUpTo)

	// Ensure that we correctly saved the processedUpTo value.
	p2 := &Processor{
		BeginningOfTime: now.Add(-24 * time.Hour),
		ChunkSize:       chunkSize,
		Name:            "test_processor",
		Frequency:       time.Millisecond,
		ProcessFn:       testProcess,
		Window:          3 * time.Hour,
		Workdir:         wd,
	}
	assert.NoError(t, p2.init())
	assert.Equal(t, now, p2.processedUpTo)

	// Test that, if we error out, we'll retry the same chunk.
	now = now.Add(30 * time.Minute)
	start = p.processedUpTo.Add(-p.Window)
	for start.Before(now) {
		end := start.Add(chunkSize)
		if now.Before(end) {
			end = now
		}
		expect = append(expect, &invocation{
			start: start,
			end:   end,
		})
		start = end
	}
	failed := expect[2]
	failed.retVal = errors.New("failed")
	expectUpTo := expect[1].end
	assert.EqualError(t, p.run(now), failed.retVal.Error())
	assert.Equal(t, expectUpTo, p.processedUpTo)
}

func TestValidate(t *testing.T) {
	testutils.SmallTest(t)

	p0 := &Processor{
		BeginningOfTime: time.Now().Add(-24 * time.Hour),
		ChunkSize:       time.Hour,
		Name:            "test_processor",
		Frequency:       time.Millisecond,
		ProcessFn:       func(time.Time, time.Time) error { return nil },
		Window:          3 * time.Hour,
		Workdir:         "my-dir",
	}
	assert.NoError(t, p0.init())

	p := new(Processor)

	// Verify that we check each of the important properties.
	*p = *p0
	p.BeginningOfTime = time.Time{}
	assert.EqualError(t, p.init(), "BeginningOfTime is required.")

	*p = *p0
	p.ChunkSize = 0
	assert.EqualError(t, p.init(), "ChunkSize is required.")

	*p = *p0
	p.Name = ""
	assert.EqualError(t, p.init(), "Name is required.")

	*p = *p0
	p.Frequency = 0
	assert.EqualError(t, p.init(), "Frequency is required.")

	*p = *p0
	p.ProcessFn = nil
	assert.EqualError(t, p.init(), "ProcessFn is required.")

	*p = *p0
	p.Window = 0
	assert.EqualError(t, p.init(), "Window is required.")

	*p = *p0
	p.Workdir = ""
	assert.EqualError(t, p.init(), "Workdir is required.")
}
