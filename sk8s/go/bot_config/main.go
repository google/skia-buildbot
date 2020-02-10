// A command-line application where each sub-command implements a get_* call in bot_config.py.
package main

import (
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/sk8s/go/bot_config/cmd"
)

func main() {
	common.Init()
	cmd.Execute()
}
