// This package defines the logging functions (e.g. Info, Errorf, etc.).

package sklog

import (
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/go/sklog/sklog_impl"
)

// WE MUST CALL SetLogger in an init function; otherwise there's a very good
// chance of getting a nil pointer panic.
func init() {
	// TODO(borenet): Switch to slog.
	sklog_impl.SetLogger(glog_and_cloud.NewLogger())
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
	sklog_impl.Log(1, sklog_impl.Debug, "", msg...)
}

func Debugf(format string, v ...interface{}) {
	sklog_impl.Log(1, sklog_impl.Debug, format, v...)
}

func DebugfWithDepth(depth int, format string, v ...interface{}) {
	sklog_impl.Log(1+depth, sklog_impl.Debug, format, v...)
}

func Info(msg ...interface{}) {
	sklog_impl.Log(1, sklog_impl.Info, "", msg...)
}

func Infof(format string, v ...interface{}) {
	sklog_impl.Log(1, sklog_impl.Info, format, v...)
}

func InfofWithDepth(depth int, format string, v ...interface{}) {
	sklog_impl.Log(1+depth, sklog_impl.Info, format, v...)
}

func Warning(msg ...interface{}) {
	sklog_impl.Log(1, sklog_impl.Warning, "", msg...)
}

func Warningf(format string, v ...interface{}) {
	sklog_impl.Log(1, sklog_impl.Warning, format, v...)
}

func WarningfWithDepth(depth int, format string, v ...interface{}) {
	sklog_impl.Log(1+depth, sklog_impl.Warning, format, v...)
}

func Error(msg ...interface{}) {
	sklog_impl.Log(1, sklog_impl.Error, "", msg...)
}

func Errorf(format string, v ...interface{}) {
	sklog_impl.Log(1, sklog_impl.Error, format, v...)
}

func ErrorfWithDepth(depth int, format string, v ...interface{}) {
	sklog_impl.Log(1+depth, sklog_impl.Error, format, v...)
}

// Fatal* exits the program after logging.
func Fatal(msg ...interface{}) {
	sklog_impl.LogAndDie(1, "", msg...)
}

func Fatalf(format string, v ...interface{}) {
	sklog_impl.LogAndDie(1, format, v...)
}

func FatalfWithDepth(depth int, format string, v ...interface{}) {
	sklog_impl.LogAndDie(1+depth, format, v...)
}

func Flush() {
	sklog_impl.Flush()
}
