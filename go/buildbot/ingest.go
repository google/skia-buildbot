package buildbot

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/go/gitinfo"
	"skia.googlesource.com/buildbot.git/go/util"
)

var (
	// TODO(borenet): Avoid hard-coding this list. Instead, obtain it from
	// checked-in code or the set of masters which are actually running.
	MASTER_NAMES = []string{"client.skia", "client.skia.android", "client.skia.compile", "client.skia.fyi"}
	httpGet      = util.NewTimeoutClient().Get
)

// get loads data from a buildbot JSON endpoint.
func get(url string, rv interface{}) error {
	resp, err := httpGet(url)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %v", err)
	}
	return nil
}

// findCommitsRecursive is a recursive function called by findCommitsForBuild.
// It traces the history to find builds which were first included in the given
// build.
func findCommitsRecursive(b *Build, hash string, repo *gitinfo.GitInfo) ([]string, error) {
	// Shortcut for empty hashes. This can happen when a commit has no
	// parents (initial commit) or when a Build has no GotRevision.
	if hash == "" {
		return []string{}, nil
	}

	// Determine whether any build already includes this commit.
	n, err := GetBuildForCommit(b.Builder, b.Master, hash)
	if err != nil {
		return nil, fmt.Errorf("Could not find build for commit %s: %v", hash, err)
	}
	// If so, stop.
	if n >= 0 {
		return []string{}, nil
	}

	// Recurse on the commit's parents.
	c, err := repo.Details(hash)
	if err != nil {
		return nil, fmt.Errorf("Failed to obtain details for %s: %v", hash, err)
	}
	commits := []string{hash}
	for _, p := range c.Parents {
		moreCommits, err := findCommitsRecursive(b, p, repo)
		if err != nil {
			return nil, err
		}
		commits = append(commits, moreCommits...)
	}
	return commits, nil
}

// findCommitsForBuild determines which commits were first included in the
// given build. Assumes that all previous builds for the given builder/master
// are already in the database.
func findCommitsForBuild(b *Build, repo *gitinfo.GitInfo) ([]string, error) {
	// Shortcut for the first build for a given builder: this build must be
	// the first inclusion for all revisions prior to b.GotRevision.
	if b.Number == 0 && b.GotRevision != "" {
		return repo.RevList(b.GotRevision)
	}
	// Start tracing commits back in time until we hit a previous build.
	return findCommitsRecursive(b, b.GotRevision, repo)
}

// getBuildFromMaster retrieves the given build from the build master's JSON
// interface as specified by the master, builder, and build number.
func getBuildFromMaster(master, builder string, buildNumber int, repo *gitinfo.GitInfo) (*Build, error) {
	var build Build
	url := fmt.Sprintf("%s%s/json/builders/%s/builds/%d", BUILDBOT_URL, master, builder, buildNumber)
	err := get(url, &build)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve build #%v for %v: %v", buildNumber, builder, err)
	}
	build.Branch = build.branch()
	build.GotRevision = build.gotRevision()
	build.Master = master
	build.Builder = builder
	slaveProp := build.GetProperty("slavename").([]interface{})
	if slaveProp != nil && len(slaveProp) == 3 {
		build.BuildSlave = slaveProp[1].(string)
	}
	build.Started = build.Times[0]
	build.Finished = build.Times[1]
	propBytes, err := json.Marshal(&build.Properties)
	if err != nil {
		return nil, fmt.Errorf("Unable to convert build properties to JSON: %v", err)
	}
	build.PropertiesStr = string(propBytes)

	// Fixup each step.
	for _, s := range build.Steps {
		if len(s.ResultsRaw) > 0 {
			if s.ResultsRaw[0] == nil {
				s.ResultsRaw[0] = 0.0
			}
			s.Results = int(s.ResultsRaw[0].(float64))
		} else {
			s.Results = 0
		}
		s.Started = s.Times[0]
		s.Finished = s.Times[1]
	}

	// Find the commits for this build.
	commits, err := findCommitsForBuild(&build, repo)
	if err != nil {
		return nil, fmt.Errorf("Could not find commits for build: %v", err)
	}
	build.Commits = commits

	return &build, nil
}

// retryGetBuildFromMaster retrieves the given build from the build master's JSON
// interface as specified by the master, builder, and build number. Makes
// multiple attempts in case the master fails to respond.
func retryGetBuildFromMaster(master, builder string, buildNumber int, repo *gitinfo.GitInfo) (*Build, error) {
	var b *Build
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		b, err = getBuildFromMaster(master, builder, buildNumber, repo)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return b, err
}

// IngestBuild retrieves the given build from the build master's JSON interface
// and pushes it into the database.
func IngestBuild(master, builder string, buildNumber int, repo *gitinfo.GitInfo) error {
	b, err := retryGetBuildFromMaster(master, builder, buildNumber, repo)
	if err != nil {
		return fmt.Errorf("Failed to load build from master: %v", err)
	}
	return b.ReplaceIntoDB()
}

// getLatestBuilds returns a map whose keys are master names and values are
// sub-maps whose keys are builder names and values are build numbers
// representing the newest build for each builder/master pair.
func getLatestBuilds() (map[string]map[string]int, error) {
	res := map[string]map[string]int{}
	errs := map[string]error{}
	type builder struct {
		CachedBuilds []int
	}
	var wg sync.WaitGroup
	for _, master := range MASTER_NAMES {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			builders := map[string]*builder{}
			err := get(BUILDBOT_URL+m+"/json/builders", &builders)
			if err != nil {
				errs[m] = fmt.Errorf("Failed to retrieve builders for %v: %v", m, err)
				return
			}
			myMap := map[string]int{}
			for name, b := range builders {
				if len(b.CachedBuilds) > 0 {
					myMap[name] = b.CachedBuilds[len(b.CachedBuilds)-1]
				}
			}
			if len(myMap) > 0 {
				res[m] = myMap
			}
		}(master)
	}
	wg.Wait()
	if len(errs) != 0 {
		return nil, fmt.Errorf("Encountered errors while loading builder data from masters: %v", errs)
	}
	return res, nil
}

// GetBuildSlaves returns a map whose keys are master names and values are
// sub-maps whose keys are slave names and values are BuildSlave objects.
func GetBuildSlaves() (map[string]map[string]*BuildSlave, error) {
	res := map[string]map[string]*BuildSlave{}
	errs := map[string]error{}
	var wg sync.WaitGroup
	for _, master := range MASTER_NAMES {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			slaves := map[string]*BuildSlave{}
			if err := get(BUILDBOT_URL+m+"/json/slaves", &slaves); err != nil {
				errs[m] = fmt.Errorf("Failed to retrieve buildslaves for %s: %v", m, err)
				return
			}
			res[m] = slaves
		}(master)
	}
	wg.Wait()
	if len(errs) != 0 {
		return nil, fmt.Errorf("Encountered errors while loading buildslave data from masters: %v", errs)
	}
	return res, nil
}

// getUningestedBuilds returns a map whose keys are master names and values are
// sub-maps whose keys are builder names and values are slices of ints
// representing the numbers of builds which have not yet been ingested.
func getUningestedBuilds() (map[string]map[string][]int, error) {
	// Get the latest and last-processed builds for all builders.
	latest, err := getLatestBuilds()
	if err != nil {
		return nil, fmt.Errorf("Failed to get latest builds: %v", err)
	}
	lastProcessed, err := getLastProcessedBuilds()
	if err != nil {
		return nil, fmt.Errorf("Failed to get last-processed builds: %v", err)
	}
	// Find the range of uningested builds for each builder.
	type numRange struct {
		Start int // The last-ingested build number.
		End   int // The latest build number.
	}
	ranges := map[string]map[string]*numRange{}
	for _, b := range lastProcessed {
		if _, ok := ranges[b.Master]; !ok {
			ranges[b.Master] = map[string]*numRange{}
		}
		ranges[b.Master][b.Builder] = &numRange{
			Start: b.Number,
			End:   b.Number,
		}
	}
	for m, v := range latest {
		if _, ok := ranges[m]; !ok {
			ranges[m] = map[string]*numRange{}
		}
		for b, n := range v {
			if _, ok := ranges[m][b]; !ok {
				ranges[m][b] = &numRange{
					Start: -1,
					End:   n,
				}
			} else {
				ranges[m][b].End = n
			}
		}
	}
	// Create a slice of build numbers for the uningested builds.
	unprocessed := map[string]map[string][]int{}
	for m, v := range ranges {
		masterMap := map[string][]int{}
		for b, r := range v {
			builds := make([]int, r.End-r.Start)
			for i := r.Start + 1; i <= r.End; i++ {
				builds[i-r.Start-1] = i
			}
			if len(builds) > 0 {
				masterMap[b] = builds
			}
		}
		if len(masterMap) > 0 {
			unprocessed[m] = masterMap
		}
	}
	return unprocessed, nil
}

// IngestNewBuilds finds the set of uningested builds and ingests them.
func IngestNewBuilds(repo *gitinfo.GitInfo) error {
	// TODO(borenet): Investigate the use of channels here. We should be
	// able to start ingesting builds as the data becomes available rather
	// than waiting until the end.
	buildsToProcess, err := getUningestedBuilds()
	if err != nil {
		return fmt.Errorf("Failed to obtain the set of uningested builds: %v", err)
	}
	unfinished, err := getUnfinishedBuilds()
	if err != nil {
		return fmt.Errorf("Failed to obtain the set of unfinished builds: %v", err)
	}
	for _, b := range unfinished {
		if _, ok := buildsToProcess[b.Master]; !ok {
			buildsToProcess[b.Master] = map[string][]int{}
		}
		if _, ok := buildsToProcess[b.Builder]; !ok {
			buildsToProcess[b.Master][b.Builder] = []int{}
		}
		buildsToProcess[b.Master][b.Builder] = append(buildsToProcess[b.Master][b.Builder], b.Number)
	}
	// TODO(borenet): Figure out how much of this is safe to parallelize.
	// We can definitely do different masters in parallel, and maybe we can
	// ingest different builders in parallel as well.
	var wg sync.WaitGroup
	errors := map[string]error{}
	for m, v := range buildsToProcess {
		wg.Add(1)
		go func(master string, buildsToProcessForMaster map[string][]int) {
			defer wg.Done()
			for b, w := range buildsToProcessForMaster {
				for _, n := range w {
					glog.Infof("Ingesting build: %s, %s, %d", master, b, n)
					if err := IngestBuild(master, b, n, repo); err != nil {
						errors[master] = fmt.Errorf("Failed to ingest build: %v", err)
					}
				}
			}
		}(m, v)
	}
	wg.Wait()
	if len(errors) > 0 {
		return fmt.Errorf("Errors: %v", errors)
	}
	return nil
}

// NumTotalBuilds finds the total number of builds which have ever run.
func NumTotalBuilds() (int, error) {
	latest, err := getLatestBuilds()
	if err != nil {
		return 0, fmt.Errorf("Failed to get latest builds: %v", err)
	}
	total := 0
	for _, m := range latest {
		for _, b := range m {
			total += b + 1 // Include build #0.
		}
	}
	return total, nil
}
