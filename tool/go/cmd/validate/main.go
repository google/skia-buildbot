// Command line application to validate configs in a given directory.
package main

import (
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/tool/go/tool"
)

var (
	directory = flag.String("dir", "./configs", "The directory where the config files are found.")
)

func main() {
	flag.Parse()
	_, messages, err := tool.LoadAndValidateFromFS(os.DirFS(*directory))
	if err != nil {
		fmt.Printf("Failed to validate configs: %s", err)
		if len(messages) > 0 {
			fmt.Println("Found the following violations:")
			for _, m := range messages {
				fmt.Println("  ", m)
			}
		}
		os.Exit(1)
	}
}
