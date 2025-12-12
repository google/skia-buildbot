package pgadapter_jar

import (
	"go.skia.org/infra/bazel/go/bazel"
)

var runfilePath = ""

// Find returns the path to the pgadapter jar from the bazel runtime directory.
func Find() (string, error) {
	return bazel.FindExecutable("pgadapter", runfilePath)
}
