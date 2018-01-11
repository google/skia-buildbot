// failures is a module for storing failed tasks and building a prediction
// model from those failures.
package failures

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/predict/go/dsconst"
	"google.golang.org/api/iterator"
)

// TaskListProvider is a type of func that returns completed failing swarming bots started between now and now-since.
type TaskListProvider func(since time.Duration) ([]*swarmingv1.SwarmingRpcsTaskRequestMetadata, error)

// BadBot is a type of func that returns true if a bot should be ignored.
type BadBot func(botname string, ts time.Time) bool

// StoredFailure is a single bot failure as stored in the datastore.
type StoredFailure struct {
	TS      time.Time
	BotName string
	Hash    string // Git hash.
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
	git        *git.Checkout
	httpClient *http.Client
	gitRepoURL string
}

func New(filter BadBot, provider TaskListProvider, git *git.Checkout, httpClient *http.Client, gitRepoURL string) *FailureStore {
	return &FailureStore{
		filter:     filter,
		provider:   provider,
		git:        git,
		httpClient: httpClient,
		gitRepoURL: gitRepoURL,
	}
}

// Update writes new StoredFailures into the datastore if any appeared between now and now-since.
func (f *FailureStore) Update(ctx context.Context, since time.Duration) error {
	resp, err := f.provider(since)
	if err != nil {
		return fmt.Errorf("Failed to query swarming: %s", err)
	}
	if err := f.git.Update(ctx); err != nil {
		return fmt.Errorf("Failed to update git repo: %s", err)
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
		startTime, err := time.Parse(swarming.TIMESTAMP_FORMAT, r.TaskResult.StartedTs)
		if err != nil {
			sklog.Infof("Failed to parse start time %v: %s", r.TaskResult.StartedTs, err)
			continue
		}
		if tags["sk_issue_server"] != "" {
			// This is from a trybot, pull the corresponding list of files from Gerrit.
			sklog.Infof("Issue: %s, Patch: %s Name: %s", tags["sk_issue"], tags["sk_patchset"], tags["sk_name"])
			url := fmt.Sprintf("%s/changes/%s/revisions/%s/files/", tags["sk_issue_server"], tags["sk_issue"], tags["sk_patchset"])
			sklog.Infof("Retrieving %q", url)
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

			files, err := f.git.Git(ctx, "show", "--pretty=", "--name-only", tags["sk_revision"])
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
		if _, err := ds.DS.Put(ctx, key, sf); err != nil {
			return fmt.Errorf("Failed to write StoredFailure to Datastore: %s", err)
		}
	}
	return nil
}

// List returns all StoredFailures in the given time range.
func (f *FailureStore) List(ctx context.Context, begin, end time.Time) ([]*StoredFailure, error) {
	ret := []*StoredFailure{}
	q := ds.NewQuery(dsconst.FAILURES).
		Filter("TS >=", begin).
		Filter("TS <", end).
		Order("TS")

	it := ds.DS.Run(ctx, q)
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

// Failures returns all the failures in the given time range.
func (f *FailureStore) Failures(ctx context.Context, begin, end time.Time) (Failures, error) {
	st, err := f.List(ctx, begin, end)
	if err != nil {
		return nil, fmt.Errorf("Failed to List from FailureStore: %s", err)
	}
	ret := Failures{}
	for _, sf := range st {
		if sf.Hash == "" {
			continue
		}
		for _, filename := range sf.Files {
			ret.Add(filename, sf.BotName)
		}
	}
	return ret, nil
}
