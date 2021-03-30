package frontend

// targetDirectories is the set of directories for which this Gazelle extension will generate or
// update BUILD files.
//
// The value of this map indicates whether to recurse into the directory.
//
// TODO(lovisolo): Delete once we are targetting the entire repository.
var targetDirectories = map[string]bool{
	"infra-sk/modules":                       false,
	"infra-sk/modules/ElementSk":             false,
	"infra-sk/modules/login-sk":              false,
	"infra-sk/modules/page_object":           false,
	"infra-sk/modules/paramset-sk":           false,
	"infra-sk/modules/query-values-sk":       false,
	"infra-sk/modules/query-sk":              false,
	"infra-sk/modules/sort-sk":               false,
	"infra-sk/modules/theme-chooser-sk":      false,
	"machine/modules":                        true,
	"machine/pages":                          true,
	"new_element/modules/example-control-sk": false,
	"perf/modules":                           true,
	"perf/pages":                             false,
	"puppeteer-tests":                        false,
}
