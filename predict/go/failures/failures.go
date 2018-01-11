// failures is a module for storing failed tasks and building a prediction
// model from those failures.
package failures

import (
	"path"
	"sort"
	"strings"

	"go.skia.org/infra/go/sklog"
)

// BotCounts is a map of bot name to the number of times it has failed.
//
// Used by Failures.
type BotCounts map[string]int

// Failures is a map of file name (or directory) to BotCounts.
type Failures map[string]BotCounts

func (f Failures) addNameOrPath(filename, botname string) {
	if bots, ok := f[filename]; !ok {
		f[filename] = map[string]int{botname: 1}
	} else {
		bots[botname] = bots[botname] + 1
	}
}

// Add the failure of botname for the given filename.
//
// Also adds all the paths walking up the tree from the filename as bot
// failures. This allows making predictions for files we've never seen before.
func (f Failures) Add(filename, botname string) {
	filename = strings.TrimSpace(filename)
	botname = strings.TrimSpace(botname)
	if filename == "" {
		return
	}
	if filename[:1] == "/" {
		// Ignore /COMMIT_MSG as it appears in every CL.
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

// predictOne returns the BotCounts for the given filename.
func (f Failures) predictOne(filename string) BotCounts {
	if strings.HasPrefix(filename, "/") {
		// Ignore /COMMIT_MSG as it appears in every CL.
		return BotCounts{}
	}
	for {
		if counts, ok := f[filename]; ok {
			return counts
		}
		filename = path.Dir(filename)
		if filename == "." {
			return BotCounts{}
		}
	}
}

// Summary is a prediction, i.e. a botname and the weight for the given bot,
// given as a count of the number of times the bot has failed for a given
// filename.
type Summary struct {
	BotName string
	Count   int
}

// SummarySlice is a utility type for sorting slices of Summary's.
//
// Summary's are sorted descending by Count, with BotName as a tiebreaker.
type SummarySlice []*Summary

func (p SummarySlice) Len() int { return len(p) }
func (p SummarySlice) Less(i, j int) bool {
	if p[i].Count == p[j].Count {
		return strings.Compare(p[i].BotName, p[j].BotName) < 0
	} else {
		return p[i].Count > p[j].Count
	}
}
func (p SummarySlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Predict which bots are most likely to fail for the given list of filenames.
func (f Failures) Predict(filenames []string) []*Summary {
	totals := BotCounts{}
	for _, filename := range filenames {
		for bot, count := range f.predictOne(filename) {
			totals[bot] += count
		}
	}
	sklog.Infof("Totals: %#v", totals)
	ordered := make([]*Summary, 0, len(totals))
	for k, v := range totals {
		ordered = append(ordered, &Summary{
			BotName: k,
			Count:   v,
		})
	}
	sort.Sort(SummarySlice(ordered))
	return ordered
}
