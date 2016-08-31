// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/lhttp"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/grpc/prpc"
)

const (
	userAgent = "luci-rpc"
)

var logCfg = gologger.LoggerConfig{
	Format: `%{message}`,
	Out:    os.Stderr,
}

// exit codes:
const (
	ecInvalidCommandLine = -iota
	ecAuthenticatedClientError
	ecOtherError
)

type exitCode struct {
	err  error
	code int
}

func (e *exitCode) Error() string { return e.err.Error() }

// cmdRun is a base of all rpc subcommands.
// It defines some common flags, such as logging and auth, and useful methods.
type cmdRun struct {
	subcommands.CommandRunBase
	cmd     *subcommands.Command
	verbose bool
	auth    authcli.Flags
}

// ModifyContext implements cli.ContextModificator.
func (r *cmdRun) ModifyContext(ctx context.Context) context.Context {
	if r.verbose {
		ctx = logging.SetLevel(ctx, logging.Debug)
	}
	return ctx
}

// registerBaseFlags registers common flags used by all subcommands.
func (r *cmdRun) registerBaseFlags() {
	r.Flags.BoolVar(&r.verbose, "verbose", false, "Enable more logging.")
	r.auth.Register(&r.Flags, auth.Options{})
}

func (r *cmdRun) authenticatedClient(ctx context.Context, host string) (*prpc.Client, error) {
	authOpts, err := r.auth.Options()
	if err != nil {
		return nil, err
	}
	a := auth.NewAuthenticator(ctx, auth.OptionalLogin, authOpts)
	httpClient, err := a.Client()
	if err != nil {
		return nil, err
	}

	client := prpc.Client{
		C:       httpClient,
		Host:    host,
		Options: prpc.DefaultOptions(),
	}
	client.Options.Insecure = lhttp.IsLocalHost(host)
	return &client, nil
}

// argErr prints an err and usage to stderr and returns an exit code.
func (r *cmdRun) argErr(format string, a ...interface{}) int {
	if format != "" {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
	fmt.Fprintln(os.Stderr, r.cmd.ShortDesc)
	fmt.Fprintln(os.Stderr, r.cmd.UsageLine)
	fmt.Fprintln(os.Stderr, "\nFlags:")
	r.Flags.PrintDefaults()
	return ecInvalidCommandLine
}

// done prints err to stderr if it is not nil and returns an exit code.
func (r *cmdRun) done(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if err, ok := err.(*exitCode); ok {
			return err.code
		}
		return ecOtherError
	}
	return 0
}

var application = &cli.Application{
	Name:  "rpc",
	Title: "Remote Procedure Call CLI",
	Context: func(ctx context.Context) context.Context {
		return logCfg.Use(ctx)
	},
	Commands: []*subcommands.Command{
		cmdCall,
		cmdShow,
		cmdFmt,
		authcli.SubcommandLogin(auth.Options{}, "login"),
		authcli.SubcommandLogout(auth.Options{}, "logout"),
		subcommands.CmdHelp,
	},
}

func main() {
	os.Exit(subcommands.Run(application, os.Args[1:]))
}
