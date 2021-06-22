package frontend

// targetDirectories is the set of directories for which this Gazelle extension will generate or
// update BUILD files.
//
// The value of this map indicates whether to recurse into the directory.
//
// TODO(lovisolo): Delete once we are targeting the entire repository.
var targetDirectories = map[string]bool{
	"golden/modules":      true,
	"golden/pages":        true,
	"infra-sk/modules":    true,
	"machine/modules":     true,
	"machine/pages":       true,
	"modules/devices":     true,
	"new_element/modules": true,
	"perf/modules":        true,
	"perf/pages":          true,
	"puppeteer-tests":     true,
	"task_driver/modules": true,
}
