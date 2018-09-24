package test_automation

import (
	"context"
	"flag"
	"net/http"
	"os"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	test   = flag.Bool("test", false, "If true, run in testing mode; send no requests, execute no commands.")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

// Main is the entry point for test_automation.
func Main(scopes []string, program func(*Context) error) {
	// TODO(borenet): Connect logging, etc.
	common.Init()
	wd, err := os.Getwd()
	if err != nil {
		sklog.Fatal(err)
	}

	// Add Receivers for step data.
	report := &ReportReceiver{}
	sc := &stepEmitter{
		receivers: map[string]Receiver{
			"DebugReceiver": &DebugReceiver{},
		},
	}
	if *output != "" {
		sc.receivers["ReportReceiver"] = report
	}

	// Initialize HTTP and exec.
	ctx := context.Background()
	var client *http.Client
	if *test {
		// Mock exec and HTTP client.
		ctx = sc.ExecCtxTesting(ctx)
		client = sc.HttpClientTesting()
	} else {
		ctx = context.Background()
		ts, err := auth.NewDefaultTokenSource(false, scopes...)
		if err != nil {
			sklog.Fatal(err)
		}
		ctx = sc.ExecCtx(ctx)
		client = sc.HttpClient(auth.ClientFromTokenSource(ts))
	}

	// Create the root Context.
	root := &rootContext{
		ctx:         ctx,
		httpClient:  client,
		stepEmitter: sc,
	}
	c := &Context{
		cwd:  wd,
		env:  nil, // TODO(borenet): Do we want to let some variables through?
		name: "root",
		root: root,
	}

	// Run the test program.
	// TODO(borenet): Add signal handlers to ensure that we send step data
	// and logs even if the program is terminated.
	if err := program(c); err != nil {
		sklog.Fatal(err)
	}

	// Dump step data if requested.
	if *output != "" {
		if *output == "-" {
			if err := report.Report(os.Stdout); err != nil {
				sklog.Fatal(err)
			}
		} else {
			if err := util.WithWriteFile(*output, report.Report); err != nil {
				sklog.Fatal(err)
			}
		}
	}
}
