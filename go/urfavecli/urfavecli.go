// Package urfavecli contains utility functions for working with https://github.com/urfave/cli.
package urfavecli

import (
	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/go/sklog"
)

// LogFlags reflects all the flags and their values as Info logs.
//
// Should be called from within the Action of a Command after logging is setup.
//
// Example
//
//      &cli.Command{
//          Name:        "my-command",
//          Action: func(c *cli.Context) error {
//              urfavecli.LogFlags(c)
//              // Do command stuff.
//          },
//      },
func LogFlags(cliContext *cli.Context) {
	for _, flag := range cliContext.App.Flags {
		name := flag.Names()[0]
		sklog.Infof("App Flags: --%s=%v", name, cliContext.Value(name))
	}
	for _, flag := range cliContext.Command.Flags {
		name := flag.Names()[0]
		sklog.Infof("Command Flags: --%s=%v", name, cliContext.Value(name))
	}
}

// MarkdownDocTemplate is a common template used to format commands as Markdown.
const MarkdownDocTemplate = `# NAME

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
