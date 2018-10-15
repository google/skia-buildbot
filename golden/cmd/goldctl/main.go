package main

// goldctl is a CLI for working with the Gold service.

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
	"go.skia.org/infra/golden/go/jsonio"
)

var (
	// Root command: goldctl itself.
	rootCmd *cobra.Command

	// Flags used throughout all commands.
	flagFile    string
	flagVerbose bool
)

func init() {
	// Set up the root command.
	rootCmd = &cobra.Command{
		Use: "goldctl",
		Long: `
goldctl interacts with the Gold service.
It can be used directly or in a scripted environment. `,
	}
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose prints out extra information")

	// validate command
	validateCmd := &cobra.Command{
		Use:     "validate",
		Aliases: []string{"va"},
		Short:   "Validate JSON",
		Long: `
Validate JSON input whether it complies with the format required for Gold
ingestion.`,
		Run: runValidateCmd,
	}
	validateCmd.Flags().StringVarP(&flagFile, "file", "f", "", "Input file to use instead of stdin")
	validateCmd.Args = cobra.NoArgs

	// Wire up the commands as children of the root command.
	rootCmd.AddCommand(validateCmd)
}

func main() {
	// Execute the root command.
	if err := rootCmd.Execute(); err != nil {
		logErrAndExit(rootCmd, err)
	}
}

// runValidateCmd implements the validation logic.
func runValidateCmd(cmd *cobra.Command, args []string) {
	f, closeFn, err := getFileOrStdin(flagFile)
	if err != nil {
		logErrfAndExit(cmd, "Error opeing input: %s", err)
	}

	goldResult, errMessages, err := jsonio.ParseGoldResults(f)
	if err != nil {
		if len(errMessages) == 0 {
			logErrfAndExit(cmd, "Error parsing JSON: %s", err)
		}

		logErr(cmd, "JSON validation failed:\n")
		for _, msg := range errMessages {
			logErrf(cmd, "   %s\n", msg)
		}
		os.Exit(1)
	}
	logExitIfError(cmd, closeFn())
	logVerbose(cmd, fmt.Sprintf("Result:\n%s\n", spew.Sdump(goldResult)))
	logVerbose(cmd, "JSON validation succeeded.\n")
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

// logExitIfError logs an error if the proviced error is not nill and exits
// with a non-zero exit code.
func logExitIfError(cmd *cobra.Command, err error) {
	if err != nil {
		logErr(cmd, err)
	}
}

// logInfo logs the given arguments based on the output settings of the command.
func logInfo(cmd *cobra.Command, args ...interface{}) {
	_, _ = fmt.Fprint(cmd.OutOrStdout(), args...)
}

// logVerbose logs the given arguments if the verbose flag is true.
func logVerbose(cmd *cobra.Command, args ...interface{}) {
	if flagVerbose {
		logInfo(cmd, args...)
	}
}
