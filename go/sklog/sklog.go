// This package defines the logging functions (e.g. Info, Errorf, etc.).

package sklog

import (
	"os"

	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
)

// WE MUST CALL SetLogger in an init function; otherwise there's a very good
// chance of getting a nil pointer panic.
func init() {
	sklogimpl.SetLogger(stdlogging.New(os.Stderr))
}

// Functions to log at various levels.
// Debug, Info, Warning, Error, and Fatal use fmt.Sprint to format the
// arguments.
// Functions ending in f use fmt.Sprintf to format the arguments.
// Functions ending in WithDepth allow the caller to change where the stacktrace
// starts. 0 (the default in all other calls) means to report starting at the
// caller. 1 would mean one level above, the caller's caller.  2 would be a
// level above that and so on.
func Debug(msg ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Debug, "", msg...)
}

func Debugf(format string, v ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Debug, format, v...)
}

func DebugfWithDepth(depth int, format string, v ...interface{}) {
	sklogimpl.Log(1+depth, sklogimpl.Debug, format, v...)
}

func Info(msg ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Info, "", msg...)
}

func Infof(format string, v ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Info, format, v...)
}

func InfofWithDepth(depth int, format string, v ...interface{}) {
	sklogimpl.Log(1+depth, sklogimpl.Info, format, v...)
}

func Warning(msg ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Warning, "", msg...)
}

func Warningf(format string, v ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Warning, format, v...)
}

func WarningfWithDepth(depth int, format string, v ...interface{}) {
	sklogimpl.Log(1+depth, sklogimpl.Warning, format, v...)
}

func Error(msg ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Error, "", msg...)
}

func Errorf(format string, v ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Error, format, v...)
}

func ErrorfWithDepth(depth int, format string, v ...interface{}) {
	sklogimpl.Log(1+depth, sklogimpl.Error, format, v...)
}

// Fatal* exits the program after logging.
func Fatal(msg ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Fatal, "", msg...)
}

func Fatalf(format string, v ...interface{}) {
	sklogimpl.Log(1, sklogimpl.Fatal, format, v...)
}

func FatalfWithDepth(depth int, format string, v ...interface{}) {
	sklogimpl.Log(1+depth, sklogimpl.Fatal, format, v...)
}

func Flush() {
	sklogimpl.Flush()
}
