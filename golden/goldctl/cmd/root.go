package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.skia.org/infra/golden/go/client"
)

var (
	// Root command: goldctl itself.
	rootCmd *cobra.Command

	// Flags
	flagFile    string
	flagVerbose bool
)

func init() {
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logErrAndExit(rootCmd, err)
	}
}

func runValidateCmd(cmd *cobra.Command, args []string) {
	f, closeFn, err := getFileOrStdin(flagFile)
	if err != nil {
		logErrfAndExit(cmd, "Error opeing input: %s", err)
	}

	if err := client.ValidateIngestionInput(f); err != nil {
		logErrfAndExit(cmd, "Validation failed: %s", err)
	}
	logVerbose(cmd, "JSON validation succeeded.\n")
	logExitIfError(cmd, closeFn())
}

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

func logErrf(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(cmd.OutOrStderr(), format, args...)
}

func logErr(cmd *cobra.Command, args ...interface{}) {
	fmt.Fprint(cmd.OutOrStderr(), args...)
}

func logErrAndExit(cmd *cobra.Command, err error) {
	logErr(cmd, err)
	os.Exit(1)
}

func logErrfAndExit(cmd *cobra.Command, format string, err error) {
	logErrf(cmd, format, err)
	os.Exit(1)
}

func logExitIfError(cmd *cobra.Command, err error) {
	if err != nil {
		logErr(cmd, err)
	}
}

func logInfo(cmd *cobra.Command, args ...interface{}) {
	fmt.Fprint(cmd.OutOrStdout(), args...)
}

func logVerbose(cmd *cobra.Command, entry string) {
	if flagVerbose {
		logInfo(cmd, entry)
	}
}
