package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Root command: goldctl itself.
	rootCmd *cobra.Command

	// Flags
	flagFile string
)

func init() {
	rootCmd = &cobra.Command{
		Use: "goldctl",
		Long: `
goldctl interacts with the Gold service.
It can be used directly or in a scripted environment. `,
	}

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
	rootCmd.AddCommand(validateCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runValidateCmd(cmd *cobra.Command, args []string) {

	fmt.Printf("Whatever .... XXX  '%s'\n", flagFile)
}
