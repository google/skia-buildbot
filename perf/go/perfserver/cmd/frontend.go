/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

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
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/frontend"
)

var flags config.Flags

// frontendCmd represents the frontend command
var frontendCmd = &cobra.Command{
	Use:   "frontend",
	Short: "The main web UI.",
	Long:  `Runs the process that serves the web UI for Perf.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("frontend called")
		f, err := frontend.New()
		if err != nil {
			sklog.Fatal(err)
		}
		f.Serve()
	},
}

func frontendInit() {
	fs := flag.NewFlagSet("frontend", flag.ContinueOnError)
	flags.Register(fs)
	frontendCmd.LocalFlags().AddGoFlagSet(fs)
	rootCmd.AddCommand(frontendCmd)
}
