package failures

import (
	"path"
	"sort"
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
	// Parse the path and also add all subpaths, which allows for giving
	// suggestions for files we've never seen before.
	for strings.Contains(filename, "/") {
		filename = path.Dir(filename)
		f.addNameOrPath(filename, botname)
	}
}

func (f Failures) predictOne(filename string) BotCounts {
	for strings.Contains(filename, "/") {
		if counts, ok := f[filename]; ok {
			return counts
		} else {
			filename = path.Dir(filename)
		}
	}
	return BotCounts{}
}

type summary struct {
	botname string
	count   int
}

type summarySlice []summary

func (p summarySlice) Len() int           { return len(p) }
func (p summarySlice) Less(i, j int) bool { return p[i].count > p[j].count }
func (p summarySlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (f Failures) Predict(filenames []string) []string {
	totals := BotCounts{}
	for _, filename := range filenames {
		for bot, count := range f.predictOne(filename) {
			totals[bot] = totals[bot] + count
		}
	}
	ordered := make([]summary, 0, len(totals))
	for k, v := range totals {
		ordered = append(ordered, summary{
			botname: k,
			count:   v,
		})
	}
	sort.Sort(summarySlice(ordered))
	ret := make([]string, 0, len(ordered))
	for _, o := range ordered {
		ret = append(ret, o.botname)
	}
	return ret
}
