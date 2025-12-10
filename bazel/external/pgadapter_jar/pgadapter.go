package pgadapter_jar

import (
	"path/filepath"

	"go.skia.org/infra/bazel/go/bazel"
)

// FindPGAdapterJar returns the path to the pgadapter jar from the bazel runtime directory.
func FindPGAdapterJar() string {
	return filepath.Join(bazel.RunfilesDir(), "external", "_main~_repo_rules~pgadapter", "pgadapter.jar")
}
