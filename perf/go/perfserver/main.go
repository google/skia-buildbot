// perfserver is the single executable that contains the sub-commands that make
// up a running Perf system, including the web ui, the ingestion process, and
// the regression detection process.
//
// This cli is built using Cobra (https://github.com/spf13/cobra/) and the cobra
// cli should be used to add new sub-commands.
package main

import "go.skia.org/infra/perf/go/perfserver/cmd"

func main() {
	cmd.Execute()
}
