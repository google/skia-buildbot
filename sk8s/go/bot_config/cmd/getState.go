package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// getStateCmd represents the getState command
var getStateCmd = &cobra.Command{
	Use:   "get_state",
	Short: "Implements get_state",
	Long: `Implements get_state in bot_config.py.

Call this and pass in a JSON dictionary that is returned
from os_utilities.get_state(), and will emit an updated
JSON dictionary on stdout.

https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/swarming_bot/config/bot_config.py`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dict := map[string]interface{}{}
		if err := json.NewDecoder(os.Stdin).Decode(&dict); err != nil {
			return fmt.Errorf("Failed to decode JSON input: %s", err)
		}

		// TODO(jcgregorio) Hook this up to Niagara.
		delete(dict, "quarantined")

		dict["sk_rack"] = os.Getenv("MY_RACK_NAME")
		if err := json.NewEncoder(os.Stdout).Encode(dict); err != nil {
			return fmt.Errorf("Failed to encode JSON output: %s", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getStateCmd)
}
