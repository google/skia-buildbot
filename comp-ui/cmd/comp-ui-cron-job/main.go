package main

import compui "go.skia.org/infra/comp-ui/go/compui"

var (
	// Key can be changed via -ldflags.
	Key = "base64 encoded service account key JSON goes here."

	// Version can be changed via -ldflags.
	Version = "unsupplied"
)

func main() {
	compui.Main(Version, Key)
}
