package logparser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSyslogParsing(t *testing.T) {
	unittest.SmallTest(t)
	contents := testutils.MustReadFile("basicsyslog")
	lp := ParseSyslog(contents)
	require.Equal(t, 2, lp.Len(), "Wrong number of log lines")

	lp.Start(0)
	if line := lp.CurrLine(); line != 0 {
		t.Errorf("Line counter should start at 0: Was %d", line)
	}
	payload := lp.ReadAndNext()
	expected := sklog.LogPayload{
		Payload:  "kernel: [ 5932.706546] usb 1-1.5.2: SerialNumber: 015d210a13480604",
		Time:     time.Date(time.Now().Year(), 5, 27, 15, 20, 15, 0, time.Local),
		Severity: sklog.INFO,
	}
	require.NotNil(t, payload)
	require.Equal(t, expected, *payload)

	if line := lp.CurrLine(); line != 1 {
		t.Errorf("Line counter should advance: Was %d", line)
	}

	payload = lp.ReadAndNext()
	expected = sklog.LogPayload{
		Payload:  "rsyslogd-2007: action 'action 17' suspended, next retry is Fri May 27 15:22:59 2016 [try http://www.rsyslog.com/e/2007 ]",
		Time:     time.Date(time.Now().Year(), 5, 27, 15, 21, 59, 0, time.Local),
		Severity: sklog.INFO,
	}
	require.NotNil(t, payload)
	require.Equal(t, expected, *payload)

	if line := lp.CurrLine(); line != 2 {
		t.Errorf("Line counter should advance: Was %d", line)
	}

	payload = lp.ReadAndNext()
	require.Nil(t, payload, "Should have reached end of input")

	if line := lp.CurrLine(); line != 2 {
		t.Errorf("Line counter should not advance: Was %d", line)
	}

	// Test ReadLine
	payload = lp.ReadLine(1)
	require.NotNil(t, payload)
	require.Equal(t, expected, *payload)
}

func TestPythonLogParsing(t *testing.T) {
	unittest.SmallTest(t)
	contents := testutils.MustReadFile("pythonlog1")
	lp := ParsePythonLog(contents)
	require.Equal(t, 5, lp.Len(), "Wrong number of log lines")

	// Spot check a few lines

	payload := lp.ReadLine(0)
	expected := sklog.LogPayload{
		Payload:  "GCE metadata not available: <urlopen error [Errno -2] Name or service not known>",
		Time:     time.Date(2016, 5, 10, 20, 01, 12, 305000000, time.UTC),
		Severity: sklog.ERROR,
	}
	require.NotNil(t, payload)
	require.Equal(t, expected, *payload)
	require.Equal(t, 0, lp.CurrLine())

	payload = lp.ReadLine(2)
	expected = sklog.LogPayload{
		Payload:  "Writing in /home/chrome-bot/.config/autostart/swarming.desktop:\n[Desktop Entry]\nType=Application\nName=swarming\nExec=/usr/bin/python /b/s/swarming_bot.zip start_bot\nHidden=false\nNoDisplay=false\nComment=Created by os_utilities.py in swarming_bot.zip\nX-GNOME-Autostart-enabled=true",
		Time:     time.Date(2016, 5, 10, 20, 01, 12, 573000000, time.UTC),
		Severity: sklog.INFO,
	}
	require.NotNil(t, payload)
	require.Equal(t, expected, *payload)
	require.Equal(t, 2, lp.CurrLine())

	payload = lp.ReadLine(3)
	expected = sklog.LogPayload{
		Payload:  "Starting new HTTPS connection (1): chromium-swarm.appspot.com",
		Time:     time.Date(2016, 5, 10, 20, 01, 12, 617000000, time.UTC),
		Severity: sklog.INFO,
	}
	require.NotNil(t, payload)
	require.Equal(t, expected, *payload)
	require.Equal(t, 3, lp.CurrLine())
}
