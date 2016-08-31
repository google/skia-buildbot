// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/flag/nestedflagset"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/system/ctxcmd"
	"github.com/luci/luci-go/common/system/environ"
	"github.com/luci/luci-go/logdog/client/bootstrapResult"
	"github.com/luci/luci-go/logdog/client/butler"
	"github.com/luci/luci-go/logdog/client/butler/bootstrap"
	"github.com/luci/luci-go/logdog/client/butler/streamserver"
	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
)

var subcommandRun = &subcommands.Command{
	UsageLine: "run",
	ShortDesc: "Bootstraps an application, passing its output through the Butler.",
	LongDesc:  "Bootstraps an application to stream through the Butler.",
	CommandRun: func() subcommands.CommandRun {
		cmd := &runCommandRun{}

		cmd.Flags.StringVar(&cmd.resultPath, "result-path", "",
			"If supplied, a JSON file describing the bootstrap result will be written here if the bootstrapped process "+
				"is successfully executed.")
		cmd.Flags.StringVar(&cmd.jsonArgsPath, "json-args-path", "",
			"If specified, this is a JSON file containing the full command to run as an "+
				"array of strings.")
		cmd.Flags.StringVar(&cmd.chdir, "chdir", "",
			"If specified, switch to this directory prior to running the command.")
		cmd.Flags.Var(&cmd.streamServerURI, "streamserver-uri",
			"The stream server URI to bind to (e.g., "+string(exampleStreamServerURI)+").")
		cmd.Flags.BoolVar(&cmd.attach, "attach", true,
			"If true, attaches the bootstrapped process' STDOUT and STDERR streams.")
		cmd.Flags.BoolVar(&cmd.stdin, "forward-stdin", false,
			"If true, forward STDIN to the bootstrapped process.")

		// "stdout" command-line option.
		cmd.stdout.Name = "stdout"
		stdoutFlag := new(nestedflagset.FlagSet)
		cmd.stdout.addFlags(&stdoutFlag.F)
		cmd.Flags.Var(stdoutFlag, "stdout", "STDOUT stream parameters.")

		// "stderr" command-line option.
		cmd.stderr.Name = "stderr"
		stderrFlag := new(nestedflagset.FlagSet)
		cmd.stderr.addFlags(&stderrFlag.F)
		cmd.Flags.Var(stderrFlag, "stderr", "STDERR stream parameters.")

		return cmd
	},
}

type runCommandRun struct {
	subcommands.CommandRunBase

	// If not empty, write bootstrap result JSON here.
	resultPath string

	// jsonArgsPath, if not empty, is the path to a JSON file containing an array
	// of strings, each of which is a command-line argument to the bootstrapped
	// command.
	jsonArgsPath string

	// chdir, if not empty, is the directory to switch to before running the
	// bootstrapped command.
	chdir string

	// streamServerURI is The path to the stream server, or empty string to
	// refrain from instantiating
	streamServerURI streamServerURI

	// attach, if true, automatically attaches the subprocess' STDOUT and STDERR
	// streams to the Butler.
	attach bool
	stdin  bool
	stdout streamConfig // Stream configuration for STDOUT.
	stderr streamConfig // Stream configuration for STDERR.
}

func (cmd *runCommandRun) Run(app subcommands.Application, args []string) int {
	a := app.(*application)

	if cmd.jsonArgsPath != "" {
		if len(args) > 0 {
			log.Errorf(a, "Cannot supply both JSON and command-line arguments.")
			return configErrorReturnCode
		}

		err := error(nil)
		args, err = cmd.loadJSONArgs()
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"path":       cmd.jsonArgsPath,
			}.Errorf(a, "Failed to load JSON arguments.")
			return configErrorReturnCode
		}

		log.Fields{
			"args": args,
			"path": cmd.jsonArgsPath,
		}.Debugf(a, "Loaded arguments from JSON file.")
	}

	if len(args) == 0 {
		log.Errorf(a, "A command must be specified.")
		return configErrorReturnCode
	}

	if cmd.streamServerURI != "" {
		if err := cmd.streamServerURI.Validate(); err != nil {
			log.Fields{
				log.ErrorKey: err,
				"uri":        cmd.streamServerURI,
			}.Errorf(a, "Invalid stream server URI.")
			return configErrorReturnCode
		}
	}

	// Get the actual path to the command
	commandPath, err := exec.LookPath(args[0])
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"command":    args[0],
		}.Errorf(a, "Failed to identify command path.")
		return runtimeErrorReturnCode
	}
	args = args[1:]

	// Verify our current working directory. "cwd" is only used for validation and
	// logging.
	cwd := cmd.chdir
	if cwd != "" {
		fi, err := os.Stat(cwd)
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"path":       cwd,
			}.Errorf(a, "Failed to stat `chdir` directory.")
			return runtimeErrorReturnCode
		}

		if !fi.IsDir() {
			log.Fields{
				"path": cwd,
			}.Errorf(a, "Target `chdir` path is not a directory.")
			return runtimeErrorReturnCode
		}
	} else {
		// Just for output.
		cwd, _ = os.Getwd()
	}

	// Update our environment for the child process to inherit
	bsEnv := bootstrap.Environment{
		Project: a.project,
		Prefix:  a.prefix,
	}

	// Configure stream server
	streamServer := streamserver.StreamServer(nil)
	streamServerOwned := true
	if cmd.streamServerURI != "" {
		log.Fields{
			"url": cmd.streamServerURI,
		}.Infof(a, "Creating stream server.")
		streamServer = createStreamServer(a, cmd.streamServerURI)

		if err := streamServer.Listen(); err != nil {
			log.Errorf(log.SetError(a, err), "Failed to connect to stream server.")
			return runtimeErrorReturnCode
		}
		defer func() {
			if streamServerOwned {
				streamServer.Close()
			}
		}()

		bsEnv.StreamServerURI = string(cmd.streamServerURI)
	}

	// Build our command enviornment.
	env := environ.System()
	bsEnv.Augment(env)

	// Construct and execute the command.
	proc := ctxcmd.CtxCmd{
		Cmd: exec.Command(commandPath, args...),
	}
	proc.Dir = cmd.chdir
	proc.Env = env.Sorted()
	if cmd.stdin {
		proc.Stdin = os.Stdin
	}

	// Attach STDOUT / STDERR pipes if configured to do so.
	//
	// We track them with a sync.WaitGroup because we have to wait for them to
	// close before reaping the process (see exec.Cmd's StdoutPipe method).
	//
	// In this case, we will hand them to the Butler, which will Close then when
	// it counters io.EOF.
	var (
		stdout, stderr io.ReadCloser
		streamWG       sync.WaitGroup
	)
	if cmd.attach {
		stdout, err = proc.StdoutPipe()
		if err != nil {
			log.WithError(err).Errorf(a, "Failed to get STDOUT pipe.")
			return runtimeErrorReturnCode
		}
		stdout = &callbackReadCloser{stdout, streamWG.Done}

		// Get our STDERR pipe
		stderr, err = proc.StderrPipe()
		if err != nil {
			log.WithError(err).Errorf(a, "Failed to get STDERR pipe.")
			return runtimeErrorReturnCode
		}
		stderr = &callbackReadCloser{stderr, streamWG.Done}
		streamWG.Add(2)
	}

	if log.IsLogging(a, log.Debug) {
		log.Fields{
			"commandPath": commandPath,
			"cwd":         cwd,
			"args":        args,
		}.Debugf(a, "Executing application.")
		for _, entry := range proc.Env {
			log.Debugf(a, "Environment variable: %s", entry)
		}
	}

	// We're about ready to execute our command. Initialize our Output instance.
	// We want to do this before we execute our subprocess so that if this fails,
	// we don't have to interrupt an already-running process.
	output, err := a.configOutput()
	if err != nil {
		log.WithError(err).Errorf(a, "Failed to create output instance.")
		return runtimeErrorReturnCode
	}
	defer output.Close()

	// Attach and run our Butler instance.
	var (
		executed   = false
		returnCode = runtimeErrorReturnCode
	)

	err = a.runWithButler(a, output, func(ctx context.Context, b *butler.Butler) error {
		// If we have configured a stream server, add it.
		if streamServer != nil {
			b.AddStreamServer(streamServer)
			streamServerOwned = false
		}

		// Add our pipes as direct streams, if configured.
		if stdout != nil {
			if err := b.AddStream(stdout, cmd.stdout.properties()); err != nil {
				return errors.Annotate(err).Reason("failed to attach STDOUT pipe stream").Err()
			}
		}
		if stderr != nil {
			if err := b.AddStream(stderr, cmd.stderr.properties()); err != nil {
				return errors.Annotate(err).Reason("failed to attach STDERR pipe stream").Err()
			}
		}

		// Execute the command. The bootstrapped application will begin executing
		// in the background.
		ctx, cancelFunc := context.WithCancel(ctx)
		if err := proc.Start(ctx); err != nil {
			return errors.Annotate(err).Reason("failed to start bootstrapped process").Err()
		}
		defer cancelFunc()

		// Wait for the process' streams to finish. We must do this before Wait()
		// on the process itself.
		streamWG.Wait()

		// Reap the process.
		err := proc.Wait()
		if rc, ok := ctxcmd.ExitCode(err); ok {
			if rc != 0 {
				log.Fields{
					"returnCode": rc,
				}.Errorf(ctx, "Command completed with non-zero return code.")
			}

			returnCode = rc
			executed = true
		} else {
			log.WithError(err).Errorf(ctx, "Command failed.")
		}

		// Wait for our Butler to finish.
		b.Activate()
		if err := b.Wait(); err != nil {
			if err == context.Canceled {
				return err
			}
			return errors.Annotate(err).Reason("failed to Wait() for Butler").Err()
		}

		return nil
	})
	if err != nil {
		logAnnotatedErr(a, err, "Error running bootstrapped Butler process:")
	}

	if !executed {
		return runtimeErrorReturnCode
	}

	// Output our bootstrap result.
	br := bootstrapResult.Result{
		ReturnCode: returnCode,
		Command:    append([]string{commandPath}, args...),
	}
	if err := cmd.maybeWriteResult(a, &br); err != nil {
		logAnnotatedErr(a, err, "Failed to write bootstrap result:")
	}

	return returnCode
}

func (cmd *runCommandRun) maybeWriteResult(ctx context.Context, r *bootstrapResult.Result) error {
	if cmd.resultPath == "" {
		return nil
	}

	log.Fields{
		"path": cmd.resultPath,
	}.Debugf(ctx, "Writing bootstrap result.")
	if err := r.WriteJSON(cmd.resultPath); err != nil {
		return errors.Annotate(err).Reason("failed to write JSON file").
			D("path", cmd.resultPath).Err()
	}
	return nil
}

func (cmd *runCommandRun) loadJSONArgs() ([]string, error) {
	fd, err := os.Open(cmd.jsonArgsPath)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	args := []string(nil)
	if err := dec.Decode(&args); err != nil {
		return nil, err
	}
	return args, nil
}

// callbackReadCloser invokes a callback method when closed.
type callbackReadCloser struct {
	io.ReadCloser
	callback func()
}

func (c *callbackReadCloser) Close() error {
	defer c.callback()
	return c.ReadCloser.Close()
}
