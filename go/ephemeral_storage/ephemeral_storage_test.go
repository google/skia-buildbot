package ephemeral_storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupForTest(t *testing.T) (func(), *bytes.Buffer) {
	t.Helper()

	// Capture os.Stdout.
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	t.Cleanup(func() {
		os.Stdout = old
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var buf bytes.Buffer

	// Store everything written to stdout.
	go func() {
		_, err := io.Copy(&buf, r)
		require.NoError(t, err)
		wg.Done()
	}()

	// Close stdout.
	waitForData := func() {
		w.Close()
		wg.Wait()
	}
	return waitForData, &buf
}

func TestUsageViaStructuredLogging_ProducesValidJSONOnStdOut(t *testing.T) {
	waitForData, buf := setupForTest(t)

	// Call UsageViaStructuredLogging.
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, UsageViaStructuredLogging(ctx))
	cancel()

	waitForData()

	// Confirm we can deserialize the output back into a `report` struct.
	var report report
	fmt.Fprintln(os.Stderr, buf.String())
	fmt.Fprintln(os.Stderr, "Hello world")
	err := json.Unmarshal(buf.Bytes(), &report)
	require.NoError(t, err)
	require.Equal(t, typeTag, report.Type)
}

func TestUsageViaStructuredLogging_ContextIsCancelled_ProducesNoOutputOnStdOut(t *testing.T) {
	waitForData, buf := setupForTest(t)

	// Call UsageViaStructuredLogging.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, UsageViaStructuredLogging(ctx))

	waitForData()

	// Confirm we can deserialize the output back into a `report` struct.
	require.Equal(t, int(0), buf.Len())
}

func TestStart_ContextIsCancelled_StartReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	Start(ctx)
	// Test will timeout if Start doesn't return.
}
