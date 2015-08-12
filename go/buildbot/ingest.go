package buildbot

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/util"
)

var (
	// TODO(borenet): Avoid hard-coding this list. Instead, obtain it from
	// checked-in code or the set of masters which are actually running.
	MASTER_NAMES = []string{"client.skia", "client.skia.android", "client.skia.compile", "client.skia.fyi"}
	httpClient   = util.NewTimeoutClient()
)

// get loads data from a buildbot JSON endpoint.
func get(url string, rv interface{}) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %v", url, err)
	}
	defer util.Close(resp.Body)
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %v", err)
	}
	return nil
}

// findCommitsRecursive is a recursive function called by FindCommitsForBuild.
// It traces the history to find builds which were first included in the given
// build.
func findCommitsRecursive(commits map[string]bool, b *Build, hash string, repo *gitinfo.GitInfo, stealFrom int, stolen []string) (map[string]bool, int, []string, error) {
	// Shortcut for empty hashes. This can happen when a commit has no
	// parents (initial commit) or when a Build has no GotRevision.
	if hash == "" {
		return commits, stealFrom, stolen, nil
	}

	// Determine whether any build already includes this commit.
	n, err := GetBuildForCommit(b.Builder, b.Master, hash)
	if err != nil {
		return commits, stealFrom, stolen, fmt.Errorf("Could not find build for commit %s: %v", hash, err)
	}
	// If so, we have to make a decision.
	if n >= 0 {
		// If the build we found is the current build, keep going,
		// since we may have already ingested data for this build but still
		// need to find accurate revision data.
		if n != b.Number {
			// If this Build's GotRevision is already included in a different
			// Build, then we're "inserting" this one in between two already-ingested
			// Builds. In that case, this build is providing "better" information
			// on the already-claimed commits, so we steal them from the other Build.
			if hash == b.GotRevision {
				stealFrom = n
			}
			if stealFrom == n {
				stolen = append(stolen, hash)
			} else {
				return commits, stealFrom, stolen, nil
			}
		}
	}

	// Add the commit.
	commits[hash] = true

	// Recurse on the commit's parents.
	c, err := repo.Details(hash)
	if err != nil {
		// Special case. Commits can disappear from the repository
		// after they're picked up by the buildbots but before they're
		// ingested here. If we can't find a commit, log an error and
		// skip the commit.
		glog.Errorf("Failed to obtain details for %s: %v", hash, err)
		delete(commits, hash)
		return commits, stealFrom, stolen, nil
	}
	for _, p := range c.Parents {
		// If we've already seen this parent commit, don't revisit it.
		if _, ok := commits[p]; ok {
			continue
		}
		commits, stealFrom, stolen, err = findCommitsRecursive(commits, b, p, repo, stealFrom, stolen)
		if err != nil {
			return commits, stealFrom, stolen, err
		}
	}
	return commits, stealFrom, stolen, nil
}

// FindCommitsForBuild determines which commits were first included in the
// given build. Assumes that all previous builds for the given builder/master
// are already in the database.
func FindCommitsForBuild(b *Build, repos *gitinfo.RepoMap) ([]string, int, []string, error) {
	if b.Repository == "" {
		return []string{}, 0, []string{}, nil
	}
	repo, err := repos.Repo(b.Repository)
	if err != nil {
		return nil, -1, nil, fmt.Errorf("Could not find commits for build: %v", err)
	}

	// Shortcut for the first build for a given builder: this build must be
	// the first inclusion for all revisions prior to b.GotRevision.
	if b.Number == 0 && b.GotRevision != "" {
		revlist, err := repo.RevList(b.GotRevision)
		return revlist, -1, []string{}, err
	}
	// Start tracing commits back in time until we hit a previous build.
	commitMap, stealFrom, stolen, err := findCommitsRecursive(map[string]bool{}, b, b.GotRevision, repo, -1, []string{})
	if err != nil {
		return nil, -1, nil, err
	}
	commits := make([]string, 0, len(commitMap))
	for c, _ := range commitMap {
		commits = append(commits, c)
	}
	return commits, stealFrom, stolen, nil
}

// getBuildFromMaster retrieves the given build from the build master's JSON
// interface as specified by the master, builder, and build number.
func getBuildFromMaster(master, builder string, buildNumber int, repos *gitinfo.RepoMap) (*Build, error) {
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
	build.Repository = build.repository()

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

	return &build, nil
}

// retryGetBuildFromMaster retrieves the given build from the build master's JSON
// interface as specified by the master, builder, and build number. Makes
// multiple attempts in case the master fails to respond.
func retryGetBuildFromMaster(master, builder string, buildNumber int, repos *gitinfo.RepoMap) (*Build, error) {
	var b *Build
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		b, err = getBuildFromMaster(master, builder, buildNumber, repos)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return b, err
}

// IngestBuild retrieves the given build from the build master's JSON interface
// and pushes it into the database.
func IngestBuild(b *Build, repos *gitinfo.RepoMap) error {
	// Find the commits for this build.
	commits, stoleFrom, stolen, err := FindCommitsForBuild(b, repos)
	if err != nil {
		return err
	}
	b.Commits = commits

	// Log the case where we found no revisions for the build.
	if !(strings.HasSuffix(b.Builder, "-Trybot") || strings.Contains(b.Builder, "Housekeeper")) && len(b.Commits) == 0 {
		glog.Infof("Got build with 0 revs: %s #%d GotRev=%s", b.Builder, b.Number, b.GotRevision)
	}
	// Determine whether we've already ingested this build. If so, fix up the ID
	// so that we update it rather than insert a new copy.
	existingBuildID, err := GetBuildIDFromDB(b.Builder, b.Master, b.Number)
	if err == nil {
		b.Id = existingBuildID
	}

	// Insert the build.
	if stoleFrom >= 0 && stolen != nil && len(stolen) > 0 {
		// Remove the commits we stole from the previous owner.
		oldBuild, err := GetBuildFromDB(b.Builder, b.Master, stoleFrom)
		if err != nil {
			return err
		}
		newCommits := make([]string, 0, len(oldBuild.Commits))
		for _, c := range oldBuild.Commits {
			keep := true
			for _, s := range stolen {
				if c == s {
					keep = false
					break
				}
			}
			if keep {
				newCommits = append(newCommits, c)
			}
		}
		oldBuild.Commits = newCommits
		return ReplaceMultipleBuildsIntoDB([]*Build{b, oldBuild})
	} else {
		return b.ReplaceIntoDB()
	}
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

// ingestNewBuilds finds the set of uningested builds and ingests them.
func ingestNewBuilds(repos *gitinfo.RepoMap) error {
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

	if err := repos.Update(); err != nil {
		return err
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
					build, err := retryGetBuildFromMaster(master, b, n, repos)
					if err != nil {
						err := fmt.Errorf("Failed to ingest build: %v", err)
						glog.Error(err)
						errors[master] = err
						return
					}
					if err := IngestBuild(build, repos); err != nil {
						err := fmt.Errorf("Failed to ingest build: %v", err)
						glog.Error(err)
						errors[master] = err
						return
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

// IngestNewBuildsLoop continually ingests new builds.
func IngestNewBuildsLoop(workdir string) {
	lv := metrics.NewLiveness("buildbot-ingest")
	repos := gitinfo.NewRepoMap(workdir)
	for _ = range time.Tick(30 * time.Second) {
		glog.Info("Ingesting builds.")
		if err := ingestNewBuilds(repos); err != nil {
			glog.Errorf("Failed to ingest new builds: %v", err)
		} else {
			lv.Update()
		}
	}
}
