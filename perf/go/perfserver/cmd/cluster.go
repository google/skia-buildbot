package cmd

import (
	"github.com/spf13/cobra"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/frontend"
)

var clusterFlags config.FrontendFlags

// clusterCmd represents the cluster command.
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Run the regression detection process.",
	Long: `Continuously runs over all the configured alerts
and looks for regressions as new data arrives.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Always to clustering.
		clusterFlags.DoClustering = true
		f, err := frontend.New(&clusterFlags, cmd.LocalFlags())
		if err != nil {
			return err
		}
		f.Serve()
		return nil
	},
}

func clusterInit() {
	rootCmd.AddCommand(clusterCmd)
	clusterFlags.Register(clusterCmd.Flags())
}
