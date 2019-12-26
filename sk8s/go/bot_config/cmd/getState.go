/*
Copyright © 2019 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
