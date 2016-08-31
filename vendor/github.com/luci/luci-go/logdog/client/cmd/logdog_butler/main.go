// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/flag/multiflag"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/common/runtime/paniccatcher"
	"github.com/luci/luci-go/logdog/client/butler"
	"github.com/luci/luci-go/logdog/client/butler/output"
	"github.com/luci/luci-go/logdog/common/types"
)

const (
	// flagErrorReturnCode is returned when there is an error with the Butler's
	// command-line configuration.
	configErrorReturnCode = 2

	// runtimeErrorReturnCode is returned when the execution fails due to a Butler
	// error. This is intended to help differentiate Butler errors from
	// passthrough bootstrapped subprocess errors.
	//
	// This will only be returned for runtime errors. If there is a flag error
	// or a configuration error, standard Butler return codes (likely to overlap
	// with standard process return codes) will be used.
	runtimeErrorReturnCode = 250
)

// application is the Butler application instance and its runtime configuration
// and state.
type application struct {
	cli.Application
	context.Context

	project       config.ProjectName
	prefix        types.StreamName
	outputWorkers int
	outputConfig  outputConfigFlag

	authFlags authcli.Flags

	maxBufferAge clockflag.Duration
	noBufferLogs bool

	cpuProfile string

	client *http.Client

	// ncCtx is a context that will not be cancelled when cancelFunc is called.
	ncCtx      context.Context
	cancelFunc func()
}

func (a *application) addFlags(fs *flag.FlagSet) {
	a.outputConfig.Output = os.Stdout
	a.outputConfig.Description = "Select and configure message output adapter."
	a.outputConfig.Options = []multiflag.Option{
		multiflag.HelpOption(&a.outputConfig.MultiFlag),
	}

	// Add registered conditional (build tag) options.
	for _, f := range getOutputFactories() {
		a.outputConfig.AddFactory(f)
	}

	a.maxBufferAge = clockflag.Duration(butler.DefaultMaxBufferAge)

	fs.Var(&a.project, "project",
		"The log prefix's project name (required).")
	fs.Var(&a.prefix, "prefix",
		"Prefix to apply to all stream names.")
	fs.Var(&a.outputConfig, "output",
		"The output name and configuration. Specify 'help' for more information.")
	fs.StringVar(&a.cpuProfile,
		"cpuprofile", "", "If specified, enables CPU profiling and profiles to the specified path.")
	fs.IntVar(&a.outputWorkers, "output-workers", butler.DefaultOutputWorkers,
		"The maximum number of parallel output dispatches.")
	fs.Var(&a.maxBufferAge, "output-max-buffer-age",
		"Send buffered messages if they've been held for longer than this period.")
	fs.BoolVar(&a.noBufferLogs, "output-no-buffer", false,
		"If true, dispatch logs immediately. Setting this flag simplifies output at the expense "+
			"of wire-format efficiency.")
}

func (a *application) authenticator(ctx context.Context) (*auth.Authenticator, error) {
	opts, err := a.authFlags.Options()
	if err != nil {
		return nil, err
	}
	return auth.NewAuthenticator(ctx, auth.SilentLogin, opts), nil
}

func (a *application) configOutput() (output.Output, error) {
	factory := a.outputConfig.getFactory()
	if factory == nil {
		return nil, errors.New("main: No output is configured")
	}

	output, err := factory.configOutput(a)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// runWithButler is an execution harness that adds application-level management
// to a Butler run.
func (a *application) runWithButler(ctx context.Context, out output.Output,
	runFunc func(context.Context, *butler.Butler) error) error {

	// Enable CPU profiling if specified
	if a.cpuProfile != "" {
		f, err := os.Create(a.cpuProfile)
		if err != nil {
			return fmt.Errorf("failed to create CPU profile output: %v", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Instantiate our Butler.
	butlerOpts := butler.Config{
		Project:       a.project,
		Prefix:        a.prefix,
		MaxBufferAge:  time.Duration(a.maxBufferAge),
		BufferLogs:    !a.noBufferLogs,
		Output:        out,
		OutputWorkers: a.outputWorkers,
		TeeStdout:     os.Stdout,
		TeeStderr:     os.Stderr,
	}
	b, err := butler.New(a, butlerOpts)
	if err != nil {
		return err
	}

	// Log the Butler's emitted streams.
	defer func() {
		if r := out.Record(); r != nil {
			// Log detail stream record.
			streams := make([]string, 0, len(r.Streams))
			for k := range r.Streams {
				streams = append(streams, string(k))
			}
			sort.Strings(streams)

			for i, stream := range streams {
				rec := r.Streams[types.StreamPath(stream)]

				ranges := make([]string, len(rec.Ranges))
				for i, rng := range rec.Ranges {
					ranges[i] = rng.String()
				}
				log.Infof(a, "%d) Stream [%s]: %s", i, stream, strings.Join(ranges, " "))
			}
		} else {
			// No record; display stream overview.
			s := b.Streams()
			paths := make([]types.StreamPath, len(s))
			for i, sn := range s {
				paths[i] = a.prefix.Join(sn)
			}
			log.Fields{
				"count":   len(paths),
				"streams": paths,
			}.Infof(a, "Butler emitted %d stream(s).", len(paths))
		}
	}()

	// Execute our Butler run function with the instantiated Butler.
	if err := runFunc(ctx, b); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(a, "Butler terminated with error.")
		a.cancelFunc()
		return err
	}

	return nil
}

// logAnnotatedErr logs the full stack trace from an annotated error to the
// installed logger at error level.
func logAnnotatedErr(ctx context.Context, err error, f string, args ...interface{}) {
	if err == nil {
		return
	}

	var buf bytes.Buffer
	st := errors.RenderStack(err)
	if _, derr := st.DumpTo(&buf); derr != nil {
		// This can't really fail, since we're rendering to a Buffer.
		panic(derr)
	}

	nargs := make([]interface{}, len(args)+1)
	nargs[copy(nargs, args)] = buf.Bytes()

	if f == "" {
		f = "Captured error stack:"
	}
	log.Errorf(ctx, f+"\n%s", nargs...)
}

func mainImpl(ctx context.Context, argv []string) int {
	authOptions := auth.Options{
		Scopes: allOutputScopes(),
	}

	a := &application{
		Context: ctx,
		Application: cli.Application{
			Name:    "butler",
			Title:   "Log collection and streaming bootstrap.",
			Context: func(context.Context) context.Context { return ctx },
			Commands: []*subcommands.Command{
				subcommands.CmdHelp,
				subcommandRun,
				subcommandStream,
				subcommandServe,

				authcli.SubcommandLogin(authOptions, "auth-login"),
				authcli.SubcommandLogout(authOptions, "auth-logout"),
				authcli.SubcommandInfo(authOptions, "auth-info"),
			},
		},
	}
	// Install logging configuration flags.
	flags := flag.NewFlagSet("flags", flag.ExitOnError)
	logConfig := log.Config{
		Level: log.Info,
	}
	logConfig.AddFlags(flags)
	a.addFlags(flags)
	a.authFlags.Register(flags, authOptions)

	// Parse the top-level flag set.
	if err := flags.Parse(argv); err != nil {
		log.WithError(err).Errorf(a, "Failed to parse command-line.")
		return configErrorReturnCode
	}

	a.Context = logConfig.Set(a.Context)

	if err := a.project.Validate(); err != nil {
		log.WithError(err).Errorf(a, "Invalid project (-project).")
		return configErrorReturnCode
	}

	// Validate our Prefix; generate a user prefix if one was not supplied.
	prefix := a.prefix
	if prefix == "" {
		// Auto-generate a prefix.
		prefix, err := generateRandomUserPrefix(a)
		if err != nil {
			log.WithError(err).Errorf(a, "Failed to generate user prefix.")
			return configErrorReturnCode
		}
		a.prefix = prefix
	}

	// Signal handler to catch 'Control-C'. This will gracefully shutdown the
	// butler the first time a signal is received. It will die abruptly if the
	// signal continues to be received.
	//
	// The specific signals used here are OS-specific.
	a.ncCtx = a.Context
	a.Context, a.cancelFunc = context.WithCancel(a.Context)
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, interruptSignals...)
	go func() {
		signalled := false
		for range signalC {
			if !signalled {
				signalled = true

				// First '^C'; soft-terminate
				log.Infof(a, "Flushing in response to Control-C (keyboard interrupt). Interrupt again for immediate exit.")
				a.cancelFunc()
			} else {
				// Multiple '^C'; terminate immediately
				os.Exit(1)
			}
		}
	}()
	defer func() {
		signal.Stop(signalC)
		close(signalC)
	}()

	log.Fields{
		"prefix": a.prefix,
	}.Infof(a, "Using session prefix.")
	if err := a.prefix.Validate(); err != nil {
		log.WithError(err).Errorf(a, "Invalid session prefix.")
		return configErrorReturnCode
	}

	// Run our subcommand (and parse subcommand flags).
	return subcommands.Run(a, flags.Args())
}

// Main execution function. This immediately jumps to 'mainImpl' and uses its
// result as an exit code.
func main() {
	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)

	// Exit with the specified return code.
	rc := 0
	defer func() {
		log.Infof(log.SetField(ctx, "returnCode", rc), "Terminating.")
		os.Exit(rc)
	}()

	paniccatcher.Do(func() {
		rc = mainImpl(ctx, os.Args[1:])
	}, func(p *paniccatcher.Panic) {
		log.Fields{
			"panic.error": p.Reason,
		}.Errorf(ctx, "Panic caught in main:\n%s", p.Stack)
		rc = runtimeErrorReturnCode
	})
}
