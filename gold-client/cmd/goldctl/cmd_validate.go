package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
	"go.skia.org/infra/golden/go/jsonio"
)

// validateEnv provides the environment for the validate command.
type validateEnv struct {
	flagFile string // flag that identifies the input file. If empty stdin will be used.
}

// getValidateCmd returns the definition of the validate command.
func getValidateCmd() *cobra.Command {
	env := &validateEnv{}

	// define the validate command
	ret := &cobra.Command{
		Use:     "validate",
		Aliases: []string{"va"},
		Short:   "Validate JSON",
		Long: `
Validate JSON input whether it complies with the format required for Gold
ingestion.`,
		Run: env.runValidateCmd,
	}
	ret.Flags().StringVarP(&env.flagFile, "file", "f", "", "Input file to use instead of stdin")
	ret.Args = cobra.NoArgs

	return ret
}

// runValidateCmd implements validation of JSON files.
func (v *validateEnv) runValidateCmd(cmd *cobra.Command, args []string) {
	f, closeFn, err := getFileOrStdin(v.flagFile)
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
	ifErrLogExit(cmd, closeFn())
	logVerbose(cmd, fmt.Sprintf("Result:\n%s\n", spew.Sdump(goldResult)))
	logVerbose(cmd, "JSON validation succeeded.\n")
}
