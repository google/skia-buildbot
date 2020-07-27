package cmd

import (
	"fmt"
	"net/http"
	"os"

	_ "net/http/pprof" // pprof

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/sklog"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "perfserver",
	Short: "The main Perf application.",
	Long: `The main Perf application.

The different parts of Perf are run as sub-commands, for example
to run the ingestion process:

	perfserver ingest --config=instance_config.json ...

`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	go func() {
		sklog.Error(http.ListenAndServe(":6060", nil))
	}()
	if err := initSubCommands(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initSubCommands() error {

	clusterInit()
	if err := ingestInit(); err != nil {
		return err
	}
	frontendInit()

	return nil
}
