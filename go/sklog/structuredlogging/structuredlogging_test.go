package structuredlogging

import (
	"context"
	"strings"
	"testing"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/structuredlogging/mocks"
	"go.skia.org/infra/go/util"
)

func TestSplitMessage(t *testing.T) {
	test := func(name, inp string, expectLengths []int) {
		t.Run(name, func(t *testing.T) {
			msgs := make([]string, 0, len(expectLengths))
			for msg := range splitMessage(inp) {
				msgs = append(msgs, msg)
			}
			require.Len(t, msgs, len(expectLengths), "expected %d results but got %d", len(expectLengths), len(msgs))

			for index, msg := range msgs {
				expectLen := expectLengths[index]
				require.Equal(t, expectLen, len(msg), "got incorrect length at index %d", index)
			}
		})
	}

	mkLine := func(length int) string {
		s := util.RandomString(length)
		// Ensure we don't have newlines.
		s = strings.ReplaceAll(s, "\n", "a")
		return s
	}

	test("empty", "", []int{0})

	noSplit := "blah blah\nblah\nblah"
	test("no split", noSplit, []int{len(noSplit)})

	longLine := mkLine(maxLogMessageBytes * 2)
	test("long line", longLine, []int{maxLogMessageBytes, maxLogMessageBytes})

	line1 := mkLine(maxLogMessageBytes - 10)
	line2 := mkLine(9)
	test("long but still one entry", line1+"\n"+line2, []int{len(line1) + len(line2) + 1})

	line2 = mkLine(10)
	inp := line1 + "\n" + line2
	require.Greater(t, len(inp), maxLogMessageBytes)
	test("two lines", line1+"\n"+line2, []int{len(line1), len(line2)})

	lines := []string{
		mkLine(10), mkLine(20), mkLine(30),
		mkLine(maxLogMessageBytes),
		mkLine(10), mkLine(maxLogMessageBytes - 11),
	}
	test("multiple lines", strings.Join(lines, "\n"), []int{
		len(lines[0]) + len(lines[1]) + len(lines[2]) + 2,
		len(lines[3]),
		len(lines[4]) + len(lines[5]) + 1,
	})
}

func TestUseWithoutInit(t *testing.T) {
	require.Nil(t, logger)
	require.Nil(t, Logger())

	ctx := context.Background()
	// None of the below should cause a nil dereference.
	Debug(ctx, "test")
	Debugf(ctx, "test %d", 123)
	DebugfWithDepth(ctx, 1, "test %d", 123)
	Info(ctx, "test")
	Infof(ctx, "test %d", 123)
	InfofWithDepth(ctx, 1, "test %d", 123)
	Warning(ctx, "test")
	Warningf(ctx, "test %d", 123)
	WarningfWithDepth(ctx, 1, "test %d", 123)
	Error(ctx, "test")
	Errorf(ctx, "test %d", 123)
	ErrorfWithDepth(ctx, 1, "test %d", 123)
	Fatal(ctx, "test")
	Fatalf(ctx, "test %d", 123)
	FatalfWithDepth(ctx, 1, "test %d", 123)
	Flush()
	logger.Flush()
	logger.Log(1, sklogimpl.Info, "test %d", 123)
	logger.LogCtx(ctx, 1, sklogimpl.Info, "test %d", 123)
}

func TestLogCtx_WithTemplate(t *testing.T) {
	ctx := WithContext(t.Context(), Context{
		Labels: map[string]string{
			"my-label": "my-value",
		},
	})
	cloudLogger := &mocks.CloudLogger{}
	logger := &StructuredLogger{
		logger: cloudLogger,
	}
	expectedLogEntry := logging.Entry{
		Payload:  "this is a template",
		Severity: logging.Error,
		// TODO(borenet): This will be very brittle.
		SourceLocation: &loggingpb.LogEntrySourceLocation{
			File:     "structuredlogging_test.go",
			Line:     120,
			Function: "",
		},
		Labels: map[string]string{
			"my-label": "my-value",
		},
	}
	cloudLogger.On("Log", expectedLogEntry).Return()
	logger.LogCtx(ctx, 0, sklogimpl.Error, "this is a %s", "template")
}

func TestLogCtx_NoTemplate_SingleArg(t *testing.T) {
	ctx := WithContext(t.Context(), Context{
		Labels: map[string]string{
			"my-label": "my-value",
		},
	})
	cloudLogger := &mocks.CloudLogger{}
	logger := &StructuredLogger{
		logger: cloudLogger,
	}
	myObject := struct {
		Prop1 string `json:"prop1"`
		Prop2 string `json:"prop2"`
	}{
		Prop1: "val1",
		Prop2: "val2",
	}
	expectedLogEntry := logging.Entry{
		Payload:  myObject,
		Severity: logging.Error,
		// TODO(borenet): This will be very brittle.
		SourceLocation: &loggingpb.LogEntrySourceLocation{
			File:     "structuredlogging_test.go",
			Line:     154,
			Function: "",
		},
		Labels: map[string]string{
			"my-label": "my-value",
		},
	}
	cloudLogger.On("Log", expectedLogEntry).Return()
	logger.LogCtx(ctx, 0, sklogimpl.Error, "", myObject)
}

func TestLogCtx_NoTemplate_MultiArg(t *testing.T) {
	ctx := WithContext(t.Context(), Context{
		Labels: map[string]string{
			"my-label": "my-value",
		},
	})
	cloudLogger := &mocks.CloudLogger{}
	logger := &StructuredLogger{
		logger: cloudLogger,
	}
	expectedLogEntry := logging.Entry{
		Payload:  "thisistextwithnotemplate",
		Severity: logging.Error,
		// TODO(borenet): This will be very brittle.
		SourceLocation: &loggingpb.LogEntrySourceLocation{
			File:     "structuredlogging_test.go",
			Line:     181,
			Function: "",
		},
		Labels: map[string]string{
			"my-label": "my-value",
		},
	}
	cloudLogger.On("Log", expectedLogEntry).Return()
	logger.LogCtx(ctx, 0, sklogimpl.Error, "", "this", "is", "text", "with", "no", "template")
}

type MyString string

func TestLogCtx_NoTemplate_NamedString(t *testing.T) {
	ctx := context.Background()
	cloudLogger := &mocks.CloudLogger{}
	logger := &StructuredLogger{
		logger: cloudLogger,
	}
	expectedLogEntry := logging.Entry{
		Payload:  "hello",
		Severity: logging.Info,
		SourceLocation: &loggingpb.LogEntrySourceLocation{
			File: "structuredlogging_test.go",
			Line: 201,
		},
	}
	cloudLogger.On("Log", expectedLogEntry).Return()
	logger.LogCtx(ctx, 0, sklogimpl.Info, "", MyString("hello"))
}

func TestLogCtx_NoTemplate_Int(t *testing.T) {
	ctx := context.Background()
	cloudLogger := &mocks.CloudLogger{}
	logger := &StructuredLogger{
		logger: cloudLogger,
	}
	expectedLogEntry := logging.Entry{
		Payload:  "123",
		Severity: logging.Info,
		SourceLocation: &loggingpb.LogEntrySourceLocation{
			File: "structuredlogging_test.go",
			Line: 219,
		},
	}
	cloudLogger.On("Log", expectedLogEntry).Return()
	logger.LogCtx(ctx, 0, sklogimpl.Info, "", 123)
}

func TestLogCtx_NoTemplate_Error(t *testing.T) {
	ctx := context.Background()
	cloudLogger := &mocks.CloudLogger{}
	logger := &StructuredLogger{
		logger: cloudLogger,
	}
	err := skerr.Fmt("my error")
	expectedLogEntry := logging.Entry{
		Payload:  err.Error(),
		Severity: logging.Info,
		SourceLocation: &loggingpb.LogEntrySourceLocation{
			File: "structuredlogging_test.go",
			Line: 238,
		},
	}
	cloudLogger.On("Log", expectedLogEntry).Return()
	logger.LogCtx(ctx, 0, sklogimpl.Info, "", err)
}

type myStringer struct{}

func (s myStringer) String() string {
	return "my stringer"
}

func TestLogCtx_NoTemplate_Stringer(t *testing.T) {
	ctx := context.Background()
	cloudLogger := &mocks.CloudLogger{}
	logger := &StructuredLogger{
		logger: cloudLogger,
	}
	s := myStringer{}
	expectedLogEntry := logging.Entry{
		Payload:  "my stringer",
		Severity: logging.Info,
		SourceLocation: &loggingpb.LogEntrySourceLocation{
			File: "structuredlogging_test.go",
			Line: 263,
		},
	}
	cloudLogger.On("Log", expectedLogEntry).Return()
	logger.LogCtx(ctx, 0, sklogimpl.Info, "", s)
}
