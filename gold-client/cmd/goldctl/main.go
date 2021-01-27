package main

// goldctl is a CLI for working with the Gold service.

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"go.skia.org/infra/gold-client/go/auth"
	"go.skia.org/infra/gold-client/go/goldclient"
)

const (
	// All commands that use a work-dir have it defined as this string.
	fstrWorkDir = "work-dir"

	exitorKey = contextKey("exitor")
)

// Flags used throughout all commands.
var (
	flagVerbose bool
	flagDryRun  bool
)

type contextKey string

func main() {
	// Set up the root command.
	rootCmd := &cobra.Command{
		Use: "goldctl",
		Long: `
goldctl interacts with the Gold service.
It can be used directly or in a scripted environment. `,
	}
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose prints out extra information")
	rootCmd.PersistentFlags().BoolVarP(&flagDryRun, "dryrun", "", false, "Dryrun causes goldctl to do everything except upload data.")

	// Wire up the other commands as children of the root command.
	rootCmd.AddCommand(getAuthCmd())
	rootCmd.AddCommand(getImgTestCmd())
	rootCmd.AddCommand(getDumpCmd())
	rootCmd.AddCommand(getDiffCmd())
	rootCmd.AddCommand(getMatchCmd())
	rootCmd.AddCommand(getWhoamiCmd())

	ctx := executionContext(context.Background(), os.Stdout, os.Stderr, os.Exit)

	// Execute the root command.
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		exitProcess(ctx, 1)
	}
}

func executionContext(ctx context.Context, log, err io.Writer, exit exitWithCode) context.Context {
	ctx = context.WithValue(ctx, goldclient.LogWriterKey, log)
	ctx = context.WithValue(ctx, goldclient.ErrorWriterKey, err)
	return context.WithValue(ctx, exitorKey, exit)
}

// logErrf logs a formatted error based on the output settings of the command.
func logErrf(ctx context.Context, format string, args ...interface{}) {
	w := ctx.Value(goldclient.ErrorWriterKey).(io.Writer)
	_, _ = fmt.Fprintf(w, format, args...)
}

// logErrfAndExit logs an error and exits with a non-zero exit code.
func logErrfAndExit(ctx context.Context, format string, err error) {
	logErrf(ctx, format, err)
	exitProcess(ctx, 1)
}

// ifErrLogExit logs an error if the proviced error is not nil and exits
// with a non-zero exit code.
func ifErrLogExit(ctx context.Context, err error) {
	if err != nil {
		logErrf(ctx, "Error running command: ''%s''\n", err)
		exitProcess(ctx, 1)
	}
}

// logInfo logs the given arguments based on the output settings of the command.
func logInfo(ctx context.Context, args ...interface{}) {
	w := ctx.Value(goldclient.LogWriterKey).(io.Writer)
	_, _ = fmt.Fprint(w, args...)
}

// logInfo logs the given arguments based on the output settings of the command.
func logInfof(ctx context.Context, format string, args ...interface{}) {
	w := ctx.Value(goldclient.LogWriterKey).(io.Writer)
	_, _ = fmt.Fprintf(w, format, args...)
}

// logVerbose logs the given arguments if the verbose flag is true.
func logVerbose(ctx context.Context, args ...interface{}) {
	if flagVerbose {
		logInfo(ctx, args...)
	}
}

type exitWithCode func(code int)

// exitProcess terminates the process with the given exit code. Note, for dry runs, we will
// typically return a zero exit code.
func exitProcess(ctx context.Context, exitCode int) {
	exit := ctx.Value(exitorKey).(exitWithCode)
	exit(exitCode)
}

// must is a helper for dealing with errors that shouldn't happen, or if they do,
// it's an error with the code, not how the user is holding it.
func must(err error) {
	if err != nil {
		fmt.Printf("Fatal startup error: %s\n", err)
		panic(err)
	}
}

func loadAuthenticatedClients(ctx context.Context, workDir string) context.Context {
	a, err := auth.LoadAuthOpt(workDir)
	ifErrLogExit(ctx, err)

	if a == nil {
		logErrf(ctx, "Auth is empty - did you call goldctl auth first?")
		exitProcess(ctx, 1)
	}
	a.SetDryRun(flagDryRun)
	gu, err := a.GetGCSUploader(ctx)
	ifErrLogExit(ctx, err)
	hc, err := a.GetHTTPClient()
	ifErrLogExit(ctx, err)
	id, err := a.GetImageDownloader()
	ifErrLogExit(ctx, err)
	return goldclient.WithContext(ctx, gu, hc, id)
}
