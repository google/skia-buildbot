package main

// goldctl is a CLI for working with the Gold service.

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Flag names used by various commands.
const (
	fstrWorkDir = "work-dir"
)

// Flags used throughout all commands.
var flagVerbose bool

func main() {
	// Set up the root command.
	rootCmd := &cobra.Command{
		Use: "goldctl",
		Long: `
goldctl interacts with the Gold service.
It can be used directly or in a scripted environment. `,
	}
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose prints out extra information")

	// Wire up the other commands as children of the root command.
	rootCmd.AddCommand(getValidateCmd())
	rootCmd.AddCommand(getAuthCmd())
	rootCmd.AddCommand(getImgTestCmd())

	// Execute the root command.
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func notImplemented(cmd *cobra.Command) {
	logErr(cmd, fmt.Errorf("Command not implemented yet."))
	os.Exit(1)
}

// getFileOrStdin returns an file to read from based on the whether file flag was set.
func getFileOrStdin(inputFile string) (*os.File, func() error, error) {
	if inputFile == "" {
		return os.Stdin, func() error { return nil }, nil
	}

	f, err := os.Open(inputFile)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// logErrf logs a formatted error based on the output settings of the command.
func logErrf(cmd *cobra.Command, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(cmd.OutOrStderr(), format, args...)
}

// logErr logs an error based on the output settings of the command.
func logErr(cmd *cobra.Command, args ...interface{}) {
	_, _ = fmt.Fprint(cmd.OutOrStderr(), args...)
}

// logErrAndExit logs a formatted error and exits with a non-zero exit code.
func logErrAndExit(cmd *cobra.Command, err error) {
	logErr(cmd, err)
	os.Exit(1)
}

// logErrfAndExit logs an error and exits with a non-zero exit code.
func logErrfAndExit(cmd *cobra.Command, format string, err error) {
	logErrf(cmd, format, err)
	os.Exit(1)
}

// ifErrLogExit logs an error if the proviced error is not nil and exits
// with a non-zero exit code.
func ifErrLogExit(cmd *cobra.Command, err error) {
	if err != nil {
		logErrf(cmd, "Error running command: ''%s''\n", err)
		os.Exit(1)
	}
}

// logInfo logs the given arguments based on the output settings of the command.
func logInfo(cmd *cobra.Command, args ...interface{}) {
	_, _ = fmt.Fprint(cmd.OutOrStdout(), args...)
}

// logInfo logs the given arguments based on the output settings of the command.
func logInfof(cmd *cobra.Command, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), format, args...)
}

// logVerbose logs the given arguments if the verbose flag is true.
func logVerbose(cmd *cobra.Command, args ...interface{}) {
	if flagVerbose {
		logInfo(cmd, args...)
	}
}
