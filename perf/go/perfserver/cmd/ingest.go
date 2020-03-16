package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Run the ingestion process.",
	Long: `Continuously imports files as they arrive from
the configured ingestion sources and populates the datastore
with that data.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("ingest called")
	},
}

func ingestInit() {
	rootCmd.AddCommand(ingestCmd)
}
