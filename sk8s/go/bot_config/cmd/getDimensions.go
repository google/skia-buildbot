package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.skia.org/infra/sk8s/go/bot_config/adb"
)

// getDimensionsCmd represents the getDimensions command by augmenting the
// current dimensions with new ones, such as which device is attached.
var getDimensionsCmd = &cobra.Command{
	Use:   "get_dimensions",
	Short: "Implements get_dimensions",
	Long: `Implements get_dimensions in bot_config.py.

Call this and pass in a JSON dictionary that is returned
from os_utilities.get_dimensions(). This command will emit
an updated JSON dictionary on stdout.

https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/swarming_bot/config/bot_config.py`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dim := map[string][]string{}
		if err := json.NewDecoder(os.Stdin).Decode(&dim); err != nil {
			return fmt.Errorf("Failed to decode JSON input: %s", err)
		}
		dim["zone"] = []string{"us", "us-skolo", "us-skolo-1"}
		dim["inside_docker"] = []string{"1", "containerd"}

		dim = adb.DimensionsFromProperties(context.Background(), cmd.ErrOrStderr(), dim)
		if err := json.NewEncoder(os.Stdout).Encode(dim); err != nil {
			return fmt.Errorf("Failed to encode JSON output: %s", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getDimensionsCmd)
}
