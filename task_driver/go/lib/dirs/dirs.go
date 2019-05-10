package dirs

import "path"

/*
	Package "dirs" contains definitions of directories used by Task Drivers.
*/

func Cache(workdir string) string {
	return path.Join(workdir, "cache")
}

func DepotTools(workdir string) string {
	return path.Join(workdir, "depot_tools")
}
