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
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.skia.org/infra/sk8s/go/bot_config/adb"
)

// getDimensionsCmd represents the getDimensions command
var getDimensionsCmd = &cobra.Command{
	Use:   "get_dimensions",
	Short: "Implements get_dimensions",
	Long: `Implements get_dimensions in bot_config.py.

Call this and pass in a JSON dictionary that is returned
from os_utilities.get_dimensions(), and will emit an updated
JSON dictionary on stdout.

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
