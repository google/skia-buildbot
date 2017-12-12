package failures

import "strings"

type BotCounts map[string]int

type Failures map[string]BotCounts

// TODO make this an atomic process.
func (f Failures) Add(filename, botname string) {
	// Note: Could parse the path and also add all subpaths,
	// which would allow for giving suggestions for files we've never seen before.
	filename = strings.TrimSpace(filename)
	botname = strings.TrimSpace(botname)
	if filename == "" {
		return
	}
	if filename[:1] == "/" {
		// Ignore /COMMIT_MSG.
		return
	}
	if bots, ok := f[filename]; !ok {
		f[filename] = map[string]int{botname: 1}
	} else {
		bots[botname] = bots[botname] + 1
	}
}

func (f Failures) Predict(filenames []string) []string {
	return nil
}
