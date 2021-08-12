// Package stdlogging implements sklogimpl.Logger and logs to either stderr or stdout.
package stdlogging

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/loggingsyncbuffer"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/testutils/unittest"
)

func testLogAtSeverity(t *testing.T, prefix, contains string, severity sklogimpl.Severity, fmt string, args ...interface{}) {
	t.Helper()
	unittest.SmallTest(t)
	sb := loggingsyncbuffer.New()
	sklogimpl.SetLogger(New(sb))
	t.Cleanup(func() {
		sklogimpl.SetLogger(New(os.Stderr))
	})

	sklogimpl.Log(1, severity, fmt, args...)

	// Don't do an exact match because log lines contain varying info like time.
	require.Contains(t, sb.String(), contains)
	require.Equal(t, prefix, sb.String()[:1])
}

func TestLog_Debugf(t *testing.T) {
	testLogAtSeverity(t, "D", "] Hello World 2!\n", sklogimpl.Debug, "Hello World %d!", 2)
}

func TestLog_Debug(t *testing.T) {
	testLogAtSeverity(t, "D", "] 2\n", sklogimpl.Debug, "", 2) // sklog.Debug because fmt is the empty string.
}

func TestLog_Infof(t *testing.T) {
	testLogAtSeverity(t, "I", "] Hello World 2!\n", sklogimpl.Info, "Hello World %d!", 2)
}

func TestLog_Info(t *testing.T) {
	testLogAtSeverity(t, "I", "] 2\n", sklogimpl.Info, "", 2) // sklog.Info because fmt is the empty string.
}

func TestLog_Warningf(t *testing.T) {
	testLogAtSeverity(t, "W", "] Hello World 2!\n", sklogimpl.Warning, "Hello World %d!", 2)
}

func TestLog_Warning(t *testing.T) {
	testLogAtSeverity(t, "W", "] 2\n", sklogimpl.Warning, "", 2)
}

func TestLog_Errorf(t *testing.T) {
	testLogAtSeverity(t, "E", "] Hello World 2!\n", sklogimpl.Error, "Hello World %d!", 2)
}

func TestLog_Error(t *testing.T) {
	testLogAtSeverity(t, "E", "] 2\n", sklogimpl.Error, "", 2)
}
