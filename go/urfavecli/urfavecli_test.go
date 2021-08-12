package urfavecli

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/go/loggingsyncbuffer"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/testutils/unittest"
)

type myGeneric struct {
	value string
}

func (m *myGeneric) Set(value string) error {
	m.value = value
	return nil
}

func (m *myGeneric) String() string {
	return m.value
}

func TestLogFlags(t *testing.T) {
	unittest.SmallTest(t)

	logsBuffer := loggingsyncbuffer.New()

	// Send logs to a buffer.
	sklogimpl.SetLogger(stdlogging.New(logsBuffer))

	commandFlags := []cli.Flag{
		&cli.BoolFlag{
			Name: "boolNotPassedIn",
		},
		&cli.BoolFlag{
			Name: "bool",
		},
		&cli.DurationFlag{
			Name: "duration",
		},
		&cli.Float64Flag{
			Name: "float64",
		},
		&cli.Float64SliceFlag{
			Name: "float64Slice",
		},
		&cli.Int64Flag{
			Name: "int64",
		},
		&cli.Int64SliceFlag{
			Name: "int64Slice",
		},
		&cli.IntFlag{
			Name: "int",
		},
		&cli.IntSliceFlag{
			Name: "intSlice",
		},
		&cli.PathFlag{
			Name: "path",
		},
		&cli.StringFlag{
			Name: "string",
		},
		&cli.StringSliceFlag{
			Name: "stringSlice",
		},
		&cli.Uint64Flag{
			Name: "uint64",
		},
		&cli.UintFlag{
			Name: "uint",
		},
	}
	app := &cli.App{
		Name: "testapp",
		Commands: []*cli.Command{
			{
				Name:  "my-command",
				Flags: commandFlags,
				Action: func(c *cli.Context) error {
					LogFlags(c)
					return nil
				},
			},
		},
	}

	// Don't print anything on stderr/stdout.
	oldHelpPrinter := cli.HelpPrinter
	cli.HelpPrinter = func(_ io.Writer, _ string, _ interface{}) {}
	defer func() {
		cli.HelpPrinter = oldHelpPrinter
	}()

	err := app.Run([]string{
		"testapp",
		"my-command",
		"--bool",
		"--duration=24s",
		"--float64=3.3",
		"--float64Slice=1.1",
		"--float64Slice=2.2",
		"--int64=64",
		"--int64Slice=128",
		"--int64Slice=129",
		"--int64Slice=130",
		"--int=65",
		"--intSlice=138",
		"--intSlice=139",
		"--intSlice=140",
		"--string=string",
		"--stringSlice=a,b,c",
		"--uint64=54",
		"--uint=53",
	})

	require.NoError(t, err)

	fullOutput := logsBuffer.String()
	lines := strings.Split(fullOutput, "\n")
	flagLines := []string{}
	for _, line := range lines {
		if strings.Contains(line, "Flags:") {
			// Strip off everything before Flags: which contains timestamps and
			// other stuff that changes.
			flagLines = append(flagLines, strings.Split(line, "Flags:")[1])
		}
	}

	expected := []string{
		" --help=false",
		" --boolNotPassedIn=false",
		" --bool=true",
		" --duration=24s",
		" --float64=3.3",
		" --float64Slice={[1.1 2.2] true}",
		" --int64=64",
		" --int64Slice={[128 129 130] true}",
		" --int=65",
		" --intSlice={[138 139 140] true}",
		" --path=",
		" --string=string",
		" --stringSlice={[a,b,c] true}",
		" --uint64=54",
		" --uint=53",
		" --help=false",
	}
	require.Equal(t, expected, flagLines)
}
