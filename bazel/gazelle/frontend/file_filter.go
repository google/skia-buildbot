package frontend

import "regexp"

// fileFilter filters files that match one or more regexps in keep, and none in skip.
type fileFilter struct {
	keep, skip []*regexp.Regexp
}

// filter filters files according to the regexps in the fileFilter struct.
func (ff *fileFilter) filter(files []string) []string {
	var kept []string

	for _, file := range files {
		willKeep := false
		for _, re := range ff.keep {
			if re.MatchString(file) {
				willKeep = true
				break
			}
		}

		willSkip := false
		for _, re := range ff.skip {
			if re.MatchString(file) {
				willSkip = true
				break
			}
		}

		if willKeep && !willSkip {
			kept = append(kept, file)
		}
	}

	return kept
}
