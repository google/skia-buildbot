// perfserver is the single executable that contains the sub-commands that make
// up a running Perf system, including the web ui, the ingestion process, and
// the regression detection process.
//
// This cli is built using Cobra (https://github.com/spf13/cobra/) and the cobra
// cli should be used to add new sub-commands.
package main

import (
	"fmt"
	"os"

	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/perf/go/perfserver/cmd"
)

func main() {
	cmd := cmd.New()
	actualMain(cmd)
}

func actualMain(cmd cmd.Commands) {}
	cli.MarkdownDocTemplate = `# NAME

{{ .App.Name }}{{ if .App.Usage }} - {{ .App.Usage }}{{ end }}

# SYNOPSIS

{{ .App.Name }}
{{ if .SynopsisArgs }}
` + "```" + `
{{ range $v := .SynopsisArgs }}{{ $v }}{{ end }}` + "```" + `
{{ end }}{{ if .App.UsageText }}
# DESCRIPTION

{{ .App.UsageText }}
{{ end }}
**Usage**:

` + "```" + `
{{ .App.Name }} [GLOBAL OPTIONS] command [COMMAND OPTIONS] [ARGUMENTS...]
` + "```" + `
{{ if .GlobalArgs }}
# GLOBAL OPTIONS
{{ range $v := .GlobalArgs }}
{{ $v }}{{ end }}
{{ end }}{{ if .Commands }}
# COMMANDS
{{ range $v := .Commands }}
{{ $v }}{{ end }}{{ end }}`

	cliApp := &cli.App{}

	cmd.Execute()

	err := cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("\nError: %s\n", err.Error())
		os.Exit(2)
	}
}
