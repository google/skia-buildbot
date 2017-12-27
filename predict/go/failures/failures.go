package failures

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/predict/go/dsconst"

	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

const TS_FORMAT = "2006-01-02T15:04:05.999999999"

// TaskListProvider is a type of func that returns completed failing swarming bots between now and now-since.
type TaskListProvider func(since time.Duration) ([]*swarmingv1.SwarmingRpcsTaskRequestMetadata, error)

// BadBot is a type of func that returns true if a bot should be ignored.
type BadBot func(botname string, ts time.Time) bool

// StoredFailure is a single bot failure.
type StoredFailure struct {
	TS      time.Time
	BotName string
	Hash    string
	Issue   string
	Patch   string
	Files   []string `datastore:",noindex"`
}

// Name of the StoredFailure, used as the key in the datastore.
func (sf *StoredFailure) Name() string {
	return fmt.Sprintf("%s-%s-%s-%s", sf.BotName, sf.Hash, sf.Issue, sf.Patch)
}

// FailureStore stores failures in the Cloud Datastore.
type FailureStore struct {
	filter     BadBot
	provider   TaskListProvider
	ctx        context.Context
	git        *git.Checkout
	httpClient *http.Client
	gitRepoURL string
}

func New(filter BadBot, provider TaskListProvider, git *git.Checkout, httpClient *http.Client, gitRepoURL string) *FailureStore {
	return &FailureStore{
		filter:     filter,
		provider:   provider,
		ctx:        context.Background(),
		git:        git,
		httpClient: httpClient,
		gitRepoURL: gitRepoURL,
	}
}

// Update writes new StoredFailures into the datastore if any appeared between now and now-since.
func (f *FailureStore) Update(since time.Duration) error {
	if err := f.git.Update(f.ctx); err != nil {
		return fmt.Errorf("Failed to update git repo: %s", err)
	}
	resp, err := f.provider(since)
	if err != nil {
		return fmt.Errorf("Failed to query swarming: %s", err)
	}
	// Used to grab the first 5 chars off of the Gerrit response which are intentional garbage.
	prefix := make([]byte, 5)
	sklog.Infof("Got %d tasks from task provider.", len(resp))
	for _, r := range resp {
		// Parse the tags.
		tags := map[string]string{}
		for _, s := range r.TaskResult.Tags {
			parts := strings.SplitN(s, ":", 2)
			tags[parts[0]] = parts[1]
		}
		if tags["sk_repo"] != f.gitRepoURL {
			sklog.Infof("Not a change for our selected repo %s != %s. %v", tags["sk_repo"], f.gitRepoURL, tags)
			continue
		}
		if strings.HasPrefix(tags["sk_name"], "Upload") {
			sklog.Info("Ingore upload bots.")
			continue
		}
		var sf *StoredFailure
		startTime, err := time.Parse(TS_FORMAT, r.TaskResult.StartedTs)
		if err != nil {
			sklog.Infof("Failed to parse start time %v: %s", r.TaskResult.StartedTs, err)
			continue
		}
		if tags["sk_issue_server"] != "" {
			// This is from a trybot, pull the corresponding list of files from Gerrit.
			sklog.Infof("Issue: %s, Patch: %s Name: %s", tags["sk_issue"], tags["sk_patchset"], tags["sk_name"])
			url := fmt.Sprintf("%s/changes/%s/revisions/%s/files/", tags["sk_issue_server"], tags["sk_issue"], tags["sk_patchset"])
			resp, err := f.httpClient.Get(url)
			if err != nil {
				sklog.Warningf("Failed to get commit file list from Gerrit: %s", err)
				continue
			}
			defer util.Close(resp.Body)
			// Trim off the first 5 chars.
			n, err := resp.Body.Read(prefix)
			if n != 5 || err != nil {
				sklog.Warningf("Failed to read file list from Gerrit: %s", err)
				continue
			}
			// The Gerrit response is a map of filenames to extra info, we only need the filenames.
			files := map[string]interface{}{}
			if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
				sklog.Warningf("Failed to get decode file list from Gerrit: %s", err)
				continue
			}
			sf = &StoredFailure{
				TS:      startTime,
				BotName: tags["sk_name"],
				Issue:   tags["sk_issue"],
				Patch:   tags["sk_patchset"],
				Hash:    "",
				Files:   []string{},
			}
			for name, _ := range files {
				sf.Files = append(sf.Files, name)
			}
		} else if tags["sk_revision"] != "" {
			// This is from the waterfall, pull the corresponding list of files from git.
			sklog.Infof("Commit: %s, Name: %s", tags["sk_revision"], tags["sk_name"])

			files, err := f.git.Git(f.ctx, "show", "--pretty=", "--name-only", tags["sk_revision"])
			if err != nil {
				sklog.Warningf("Failed to get commit file list: %s", err)
				continue
			}

			sf = &StoredFailure{
				TS:      startTime,
				BotName: tags["sk_name"],
				Issue:   "",
				Patch:   "",
				Hash:    tags["sk_revision"],
				Files:   []string{},
			}
			for _, filename := range strings.Split(files, "\n") {
				filename = strings.TrimSpace(filename)
				if filename != "" {
					sf.Files = append(sf.Files, filename)
				}
			}
		} else {
			sklog.Info("An unknown type of task, possible a leased device task.")
			continue
		}
		if f.filter(sf.BotName, sf.TS) {
			sklog.Infof("Filtered: %s", sf.BotName)
			continue
		}
		key := ds.NewKey(dsconst.FAILURES)
		key.Name = sf.Name()
		if _, err := ds.DS.Put(f.ctx, key, sf); err != nil {
			return fmt.Errorf("Failed to write StoredFailure to Datastore: %s", err)
		}
	}
	return nil
}

// List returns all StoredFailures in the given time range.
func (f *FailureStore) List(begin, end time.Time) ([]*StoredFailure, error) {
	ret := []*StoredFailure{}
	q := ds.NewQuery(dsconst.FAILURES).Filter("TS >=", begin).Filter("TS <", end).Order("TS")
	it := ds.DS.Run(f.ctx, q)
	for {
		row := &StoredFailure{}
		_, err := it.Next(row)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed loading StoredFailure: %s", err)
		}
		ret = append(ret, row)
	}
	return ret, nil
}

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
