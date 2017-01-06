package storage

import "strings"

// ExtractFuzzNameFromPath turns a path name into a fuzz name by stripping off all but the
// last piece from the path.
func ExtractFuzzNameFromPath(path string) (name string) {
	return path[strings.LastIndex(path, "/")+1:]
}

// ExtractFuzzNamesFromPaths turns all path names into just fuzz names, by extracting the
// last piece of the path.
func ExtractFuzzNamesFromPaths(paths []string) (names []string) {
	names = make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, ExtractFuzzNameFromPath(path))
	}
	return names
}

// IsNameOfFuzz returns true if the GCS file name given is a fuzz, which is basically if it doesn't
// have a . in it.
func IsNameOfFuzz(name string) bool {
	return name != "" && !strings.Contains(name, ".")
}
