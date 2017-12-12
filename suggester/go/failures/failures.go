package failures

import (
	"path"
	"strings"
)

type BotCounts map[string]int

type Failures map[string]BotCounts

func (f Failures) addNameOrPath(filename, botname string) {
	if bots, ok := f[filename]; !ok {
		f[filename] = map[string]int{botname: 1}
	} else {
		bots[botname] = bots[botname] + 1
	}
}

func (f Failures) Add(filename, botname string) {
	filename = strings.TrimSpace(filename)
	botname = strings.TrimSpace(botname)
	if filename == "" {
		return
	}
	if filename[:1] == "/" {
		// Ignore /COMMIT_MSG.
		return
	}
	f.addNameOrPath(filename, botname)
	// Parse the path and also add all subpaths,
	// which would allow for giving suggestions for files we've never seen before.
	for strings.Contains(filename, "/") {
		filename = path.Dir(filename)
		f.addNameOrPath(filename, botname)
	}
}

func (f Failures) Predict(filenames []string) []string {
	return nil
}
