// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/common/system/ctxcmd"

	"golang.org/x/net/context"
)

const (
	workExecDumpLineCount   = 10
	workExecDumpLineTimeout = 5 * time.Second
)

type work struct {
	context.Context
	parallel.MultiRunner
	*tools
}

type workExecutor struct {
	command string
	args    []string
	workdir string
	envMap  map[string]string

	outputLevel         log.Level
	shouldForwardOutput bool

	stdout bytes.Buffer
	stderr bytes.Buffer
}

func execute(cmd string, args ...string) *workExecutor {
	return &workExecutor{
		command:     cmd,
		args:        args,
		outputLevel: log.Debug,
	}
}

func (x *workExecutor) bootstrap(command string, args ...string) *workExecutor {
	nargs := make([]string, 0, 1+len(args)+len(x.args))
	nargs = append(append(append(nargs, args...), x.command), x.args...)
	x.command, x.args = command, nargs
	return x
}

func (x *workExecutor) cwd(path string) *workExecutor {
	x.workdir = path
	return x
}

func (x *workExecutor) loadEnv(e []string) *workExecutor {
	for _, v := range e {
		switch parts := strings.SplitN(v, "=", 2); len(parts) {
		case 1:
			x.env(parts[0], "")

		case 2:
			x.env(parts[0], parts[1])
		}
	}
	return x
}

func (x *workExecutor) env(key string, value string) *workExecutor {
	if x.envMap == nil {
		x.envMap = make(map[string]string)
	}
	x.envMap[key] = value
	return x
}

func (x *workExecutor) envPath(key string, value ...string) *workExecutor {
	return x.env(key, strings.Join(value, string(os.PathListSeparator)))
}

func (x *workExecutor) forwardOutput() *workExecutor {
	x.shouldForwardOutput = true
	return x
}

func (x *workExecutor) outputAt(l log.Level) *workExecutor {
	x.outputLevel = l
	return x
}

func (x *workExecutor) run(c context.Context) (int, error) {
	// Clear our buffers for this command.
	x.stdout.Reset()
	x.stderr.Reset()

	// Setup / execute the command.
	cmd := ctxcmd.CtxCmd{
		Cmd: exec.Command(x.command, x.args...),
	}
	cmd.Dir = x.workdir

	// Setup pipes and goroutines to dump pipes periodically so we can see
	// what's happening.
	var wg sync.WaitGroup
	switch {
	case x.shouldForwardOutput:
		cmd.Stdout = &teeWriter{os.Stdout, &x.stdout}
		cmd.Stderr = &teeWriter{os.Stderr, &x.stderr}

	case log.IsLogging(c, x.outputLevel):
		// Get our command pipes. Wrap each one in a "closeOnceReader" so that our
		// reader goroutine can close on error and our outer loop can also close on
		// error without conflicting.
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return -1, errors.Annotate(err).Reason("failed to create STDOUT pipe").Err()
		}
		stdoutPipe = &closeOnceReader{ReadCloser: stdoutPipe}
		defer stdoutPipe.Close()

		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return -1, errors.Annotate(err).Reason("failed to create STDERR pipe").Err()
		}
		stderrPipe = &closeOnceReader{ReadCloser: stderrPipe}
		defer stderrPipe.Close()

		spawnMonitor := func(name string, in io.ReadCloser, tee io.Writer) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer in.Close()

				var (
					tr       = io.TeeReader(in, tee)
					reader   = bufio.NewReader(tr)
					lastDump = clock.Now(c)
					lines    = make([]string, 0, workExecDumpLineCount)
				)

				dump := func(now time.Time) {
					if len(lines) > 0 {
						log.Logf(c, x.outputLevel, "Command %s %s output %s:\n%s",
							x.command, x.args, name, strings.Join(lines, ""))
					}
					lines = lines[:0]
					lastDump = now
				}

				for {
					line, err := reader.ReadString('\n')
					if len(line) > 0 {
						lines = append(lines, line)
					}
					if err != nil {
						break
					}

					dumpThreshold := lastDump.Add(workExecDumpLineTimeout)
					now := clock.Now(c)
					if len(lines) >= workExecDumpLineCount || dumpThreshold.Before(now) {
						dump(now)
					}
				}
				dump(time.Time{})
			}()
		}
		spawnMonitor("stdout", stdoutPipe, &x.stdout)
		spawnMonitor("stderr", stderrPipe, &x.stderr)

	default:
		// We wouldn't see the logs anyway, so buffer directly.
		cmd.Stdout = &x.stdout
		cmd.Stderr = &x.stderr
	}

	if len(x.envMap) > 0 {
		// Get a sorted list of keys (determinism).
		env := make([]string, 0, len(x.envMap))
		for k := range x.envMap {
			env = append(env, k)
		}
		sort.Strings(env)

		// Replace with environment.
		for i, k := range env {
			env[i] = fmt.Sprintf("%s=%s", k, x.envMap[k])
		}
		cmd.Env = env
	}

	log.Fields{
		"cwd": x.workdir,
	}.Debugf(c, "Running command: %s %s.", x.command, x.args)
	if err := cmd.Start(c); err != nil {
		return -1, errors.Annotate(err).Reason("failed to start command").
			D("command", x.command).D("args", x.args).D("cwd", x.workdir).Err()
	}

	// Wait for our stream processing to finish.
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if rc, ok := ctxcmd.ExitCode(err); ok {
			log.Fields{
				"returnCode": rc,
			}.Debugf(c, "Command completed with non-zero return code: %s %s", x.command, x.args)
			return rc, nil
		}

		return -1, errors.Annotate(err).Reason("failed to wait for command").
			D("command", x.command).D("args", x.args).D("cwd", x.workdir).Err()
	}

	log.Debugf(c, "Command completed with zero return code: %s %s", x.command, x.args)
	return 0, nil
}

func (x *workExecutor) check(c context.Context) error {
	switch rc, err := x.run(c); {
	case err != nil:
		return errors.Annotate(err).Err()

	case rc != 0:
		log.Fields{
			"returnCode": rc,
			"command":    x.command,
			"args":       x.args,
			"cwd":        x.workdir,
		}.Errorf(c, "Command failed with error return code.\nSTDOUT:\n%s\n\nSTDERR:\n%s",
			x.stdout.String(), x.stderr.String())
		return errors.Reason("process exited with return code: %(returnCode)d").
			D("command", x.command).D("args", x.args).D("cwd", x.workdir).D("returnCode", rc).Err()

	default:
		return nil
	}
}

func runWork(c context.Context, workers int, tools *tools, f func(w *work) error) error {
	return parallel.RunMulti(c, workers, func(mr parallel.MultiRunner) error {
		return f(&work{
			Context:     c,
			tools:       tools,
			MultiRunner: mr,
		})
	})
}

type closeOnceReader struct {
	io.ReadCloser
	close sync.Once
}

func (r *closeOnceReader) Close() (err error) {
	r.close.Do(func() {
		err = r.ReadCloser.Close()
	})
	return
}

func addGoEnv(goPath []string, x *workExecutor) *workExecutor {
	return x.loadEnv(os.Environ()).envPath("GOPATH", goPath...)
}

// teeWriter writes data to the base writer, then to the supplied tee writer.
// The output of the base Write will be returned. If the tee write failed, Write
// will panic.
type teeWriter struct {
	base io.Writer
	tee  io.Writer
}

func (w *teeWriter) Write(d []byte) (amt int, err error) {
	amt, err = w.base.Write(d)
	if amt > 0 {
		if _, ierr := w.tee.Write(d[:amt]); ierr != nil {
			panic(errors.Annotate(ierr).Reason("failed to write to tee writer").Err())
		}
	}
	return
}
