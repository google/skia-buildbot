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

type isolated struct {
	Size int64 `json:"size"`
}

type caches struct {
	Isolated isolated `json:"isolated"`
}

type settings struct {
	Caches caches `json:"caches"`
}

// getSettingsCmd represents the getSettings command
var getSettingsCmd = &cobra.Command{
	Use:   "get_settings",
	Short: "Implements get_settings.",
	Long: `Implements get_settings for bot_config.py

Will emit a JSON dictionary on stdout with the settings.

	https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/swarming_bot/config/bot_config.py`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dict := settings{
			caches{
				isolated{
					Size: 8 * 1024 * 1024 * 1024,
				},
			},
		}
		if err := json.NewEncoder(os.Stdout).Encode(dict); err != nil {
			return fmt.Errorf("Failed to encode JSON output: %s", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getSettingsCmd)
}
