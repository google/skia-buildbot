package dirs

import (
	"path/filepath"
)

/*
	Package "dirs" contains definitions of directories used by Task Drivers.
*/

func Cache(workdir string) string {
	return filepath.Join(workdir, "cache")
}

func DepotTools(workdir string) string {
	return filepath.Join(workdir, "depot_tools")
}
