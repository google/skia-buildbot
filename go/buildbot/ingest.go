package buildbot

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

const (
	MAX_BLAMELIST_COMMITS = 500
)

var (
	// BUILD_BLACKLIST is a set of builds which, for one reason or another,
	// we want to skip during ingestion. Typically this means that there is
	// something wrong with the build which prevents it from being ingested
	// properly.
	BUILD_BLACKLIST = map[string]map[int]bool{
		"Build-Mac10.9-Clang-x86_64-Debug": map[int]bool{
			5222: true, // This build doesn't exist on the server.
		},
		"Build-Mac10.9-Clang-x86_64-Release": map[int]bool{
			5207: true, // This build doesn't exist on the server.
		},
		"Build-Mac10.9-Clang-x86_64-Release-CMake": map[int]bool{
			891: true, // This build doesn't exist on the server.
		},
		// Something went haywire with this, don't know what. -dogben
		"Build-Ubuntu-GCC-x86-Release": map[int]bool{
			2586: true,
		},
		"Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release-BuildBucket": map[int]bool{
			1: true, // Cannot be ingested because its repo is "???"
		},
		"Perf-Ubuntu-GCC-ShuttleA-GPU-GTX660-x86_64-Release-ANGLE": map[int]bool{
			350: true, // This bot was removed before this build finished ingesting.
		},
		"Perf-Ubuntu-GCC-ShuttleA-GPU-GTX660-x86_64-Release-VisualBench": map[int]bool{
			0: true, // Wrong repo.
			2: true, // Wrong repo.
			3: true, // Wrong repo.
		},
		// This bot was removed before these build finished ingesting.
		"Perf-Win8-MSVC-ShuttleB-CPU-AVX2-x86_64-Release-Swarming": map[int]bool{
			12510: true,
			12511: true,
		},
		"Linux Tests": map[int]bool{
			// For some reason, these builds don't exist on the server.
			2872: true,
			2920: true,
			2995: true,
			3144: true,
			3193: true,
			3197: true,
		},
		"Mac10.9 Tests": map[int]bool{
			1727: true, // This build doesn't exist on the server.
		},
		// This bot was removed before these build finished ingesting.
		"Test-Win8-MSVC-ShuttleB-CPU-AVX2-x86_64-Release-Swarming": map[int]bool{
			12588: true,
			12589: true,
			12590: true,
		},
		"Win7 Tests (1)": map[int]bool{
			1797: true, // This build doesn't exist on the server?
		},
		// This bot was removed before the build finished ingesting.
		"Test-Ubuntu-GCC-ShuttleA-GPU-GTX550Ti-x86_64-Release-SwarmingValgrind": map[int]bool{
			107: true,
		},
	}

	// TODO(borenet): Avoid hard-coding this list. Instead, obtain it from
	// checked-in code or the set of masters which are actually running.
	MASTER_NAMES = []string{"client.skia", "client.skia.android", "client.skia.compile", "client.skia.fyi"}
	httpClient   = httputils.NewTimeoutClient()
)

// get loads data from a buildbot JSON endpoint.
func get(url string, rv interface{}) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %s", url, err)
	}
	defer util.Close(resp.Body)
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return nil
}

// findCommitsRecursive is a recursive function called by FindCommitsForBuild.
// It traces the history to find builds which were first included in the given
// build.
func findCommitsRecursive(db DB, commits map[string]bool, b *Build, hash string, repo *gitinfo.GitInfo, stealFrom int, stolen []string) (map[string]bool, int, []string, error) {
	// Shortcut for empty hashes. This can happen when a commit has no
	// parents (initial commit) or when a Build has no GotRevision.
	if hash == "" {
		return commits, stealFrom, stolen, nil
	}

	// Shortcut in case we missed this case before; if this is the first
	// build on this bot which has a valid GotRevision, the blamelist will
	// be the entire Git history. If we find too many commits, assume we've
	// hit this case and just return the GotRevision as the blamelist.
	if len(commits) > MAX_BLAMELIST_COMMITS && stealFrom == -1 {
		return map[string]bool{b.GotRevision: true}, -1, []string{}, nil
	}

	// Determine whether any build already includes this commit.
	n, err := db.GetBuildNumberForCommit(b.Master, b.Builder, hash)
	if err != nil {
		return commits, stealFrom, stolen, fmt.Errorf("Could not find build for commit %s: %s", hash, err)
	}

	// If we're stealing commits from a previous build but the current
	// commit is not in any build's blamelist, we must have scrolled past
	// the beginning of the builds. Just return.
	if n < 0 && stealFrom >= 0 {
		return commits, stealFrom, stolen, nil
	}

	// If a previous build already included this commit, we have to make a decision.
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
				// Another shortcut: If our GotRevision is the same as the
				// GotRevision of the Build we're stealing commits from,
				// ie. both builds ran at the same commit, just take all of
				// its commits without doing any more work.
				stealFromBuild, err := db.GetBuildFromDB(b.Master, b.Builder, stealFrom)
				if err != nil {
					return commits, stealFrom, stolen, fmt.Errorf("Could not retrieve build: %s", err)
				}
				if stealFromBuild.GotRevision == b.GotRevision && stealFromBuild.Number < b.Number {
					commits = map[string]bool{}
					for _, c := range stealFromBuild.Commits {
						commits[c] = true
					}
					return commits, stealFrom, stealFromBuild.Commits, nil
				}
			}
			if stealFrom == n {
				// Continue stealing commits from the older build.
				stolen = append(stolen, hash)
			} else {
				// If we've hit a commit belonging to a different build,
				// just return.
				return commits, stealFrom, stolen, nil
			}
		}
	}

	// Add the commit.
	commits[hash] = true

	// Recurse on the commit's parents.
	c, err := repo.Details(hash, false)
	if err != nil {
		// Special case. Commits can disappear from the repository
		// after they're picked up by the buildbots but before they're
		// ingested here. If we can't find a commit, log an error and
		// skip the commit.
		glog.Errorf("Failed to obtain details for %s: %s", hash, err)
		delete(commits, hash)
		return commits, stealFrom, stolen, nil
	}
	for _, p := range c.Parents {
		// If we've already seen this parent commit, don't revisit it.
		if _, ok := commits[p]; ok {
			continue
		}
		commits, stealFrom, stolen, err = findCommitsRecursive(db, commits, b, p, repo, stealFrom, stolen)
		if err != nil {
			return commits, stealFrom, stolen, err
		}
	}
	return commits, stealFrom, stolen, nil
}

// FindCommitsForBuild determines which commits were first included in the
// given build. Assumes that all previous builds for the given builder/master
// are already in the database.
func FindCommitsForBuild(db DB, b *Build, repos *gitinfo.RepoMap) ([]string, int, []string, error) {
	defer metrics2.FuncTimer().Stop()
	// Shortcut: Don't bother computing commit blamelists for trybots.
	if IsTrybot(b.Builder) {
		return []string{}, -1, []string{}, nil
	}
	if b.Repository == "" {
		return []string{}, -1, []string{}, nil
	}
	repo, err := repos.Repo(b.Repository)
	if err != nil {
		return nil, -1, nil, fmt.Errorf("Could not find commits for build: %s", err)
	}

	// Update (git pull) on demand.
	if b.GotRevision != "" {
		if _, err := repo.Details(b.GotRevision, false); err != nil {
			if err := repo.Update(true, true); err != nil {
				return nil, -1, nil, fmt.Errorf("Could not find commits for build: failed to update repo: %s", err)
			}
		}
	}

	// Shortcut for the first build for a given builder: Just use GotRevision
	// as the blamelist.
	if b.Number == 0 && b.GotRevision != "" {
		return []string{b.GotRevision}, -1, []string{}, nil
	}
	// Start tracing commits back in time until we hit a previous build.
	commitMap, stealFrom, stolen, err := findCommitsRecursive(db, map[string]bool{}, b, b.GotRevision, repo, -1, []string{})
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
		return nil, fmt.Errorf("Failed to retrieve build #%d for %s: %s", buildNumber, builder, err)
	}
	build.fixup()
	if build.Repository == "" {
		// Attempt to determine the repository.
		glog.Infof("No repository set for %s #%d; attempting to find it.", build.Builder, build.Number)
		r, err := repos.RepoForCommit(build.GotRevision)
		if err == nil {
			glog.Infof("Found %s for %s", r, build.GotRevision)
			build.Repository = r
		} else {
			glog.Warningf("Encountered error determining repo for %s: %s", build.GotRevision, err)
		}
	}

	return &build, nil
}

// retryGetBuildFromMaster retrieves the given build from the build master's JSON
// interface as specified by the master, builder, and build number. Makes
// multiple attempts in case the master fails to respond.
func retryGetBuildFromMaster(master, builder string, buildNumber int, repos *gitinfo.RepoMap) (*Build, error) {
	defer metrics2.FuncTimer().Stop()
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

// validateBuildForIngestion verifies that the build is ready to be ingested.
func validateBuildForIngestion(b *Build) error {
	if b.Master == "" {
		return fmt.Errorf("Build has no master name!")
	}
	if b.Builder == "" {
		return fmt.Errorf("Build has no builder name!")
	}
	if util.TimeIsZero(b.Started) {
		return fmt.Errorf("Build has no start time!")
	}
	return nil
}

// IngestBuild retrieves the given build from the build master's JSON interface
// and pushes it into the database.
func IngestBuild(db DB, b *Build, repos *gitinfo.RepoMap) error {
	defer metrics2.FuncTimer().Stop()
	if err := validateBuildForIngestion(b); err != nil {
		return err
	}
	// Find the previously-inserted version of this build, if it exists,
	// and update it rather than inserting a brand new build.
	needToComputeBlamelist := true
	oldBuild, err := db.GetBuild(b.Id())
	if err == nil {
		if oldBuild.GotRevision == "" {
			oldBuild.GotRevision = b.GotRevision
		} else {
			needToComputeBlamelist = false
		}
		if b.GotRevision != oldBuild.GotRevision {
			return fmt.Errorf("Cannot change an already-ingested build's GotRevision.")
		}
		oldBuild.Results = b.Results
		oldBuild.Properties = b.Properties
		oldBuild.PropertiesStr = b.PropertiesStr
		oldBuild.Steps = b.Steps
		oldBuild.Finished = b.Finished

		b = oldBuild
	}
	// Record metrics for unfinished buildsteps.
	if !b.IsFinished() {
		now := time.Now()
		for _, s := range b.Steps {
			if s.IsStarted() && !s.IsFinished() {
				runTime := now.Sub(s.Started)
				metrics2.RawAddInt64PointAtTime("buildbot.buildsteps.running-time", map[string]string{
					"builder":    b.Builder,
					"buildslave": b.BuildSlave,
					"master":     b.Master,
					"number":     strconv.Itoa(b.Number),
					"step":       s.Name,
				}, int64(runTime), now)
			}
		}
	}
	if needToComputeBlamelist {
		// Find the commits for this build.
		commits, stoleFrom, stolen, err := FindCommitsForBuild(db, b, repos)
		if err != nil {
			return err
		}
		b.Commits = commits

		// Log the case where we found no revisions for the build.
		if !(IsTrybot(b.Builder) || strings.Contains(b.Builder, "Housekeeper")) && len(b.Commits) == 0 {
			glog.Infof("Got build with 0 revs: %s #%d GotRev=%s", b.Builder, b.Number, b.GotRevision)
		}

		// Insert the build.
		if stoleFrom >= 0 && stolen != nil && len(stolen) > 0 {
			// Remove the commits we stole from the previous owner.
			oldBuild, err := db.GetBuildFromDB(b.Master, b.Builder, stoleFrom)
			if err != nil {
				return err
			}
			if oldBuild == nil {
				return fmt.Errorf("Attempted to retrieve %s #%d, but got a nil build from the DB.", b.Builder, stoleFrom)
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
			return db.PutBuilds([]*Build{b, oldBuild})
		}
	}
	return db.PutBuild(b)
}

// getLatestBuilds returns a map whose keys are master names and values are
// sub-maps whose keys are builder names and values are build numbers
// representing the newest build for each builder/master pair.
func getLatestBuilds(m string) (map[string]int, error) {
	type builder struct {
		CachedBuilds []int
	}
	builders := map[string]*builder{}
	if err := get(BUILDBOT_URL+m+"/json/builders", &builders); err != nil {
		return nil, fmt.Errorf("Failed to retrieve builders for %s: %s", m, err)
	}
	res := map[string]int{}
	for name, b := range builders {
		if len(b.CachedBuilds) > 0 {
			res[name] = b.CachedBuilds[len(b.CachedBuilds)-1]
		}
	}
	return res, nil
}

// GetBuilders returns the set of builders from all masters.
func GetBuilders() (map[string]*Builder, error) {
	var mtx sync.Mutex
	builders := map[string][]*Builder{}
	errs := map[string]error{}
	var wg sync.WaitGroup
	for _, m := range MASTER_NAMES {
		wg.Add(1)
		go func(master string) {
			defer wg.Done()
			b := map[string]*Builder{}
			err := get(BUILDBOT_URL+master+"/json/builders", &b)
			mtx.Lock()
			defer mtx.Unlock()
			if err != nil {
				errs[master] = err
				return
			}
			builderList := make([]*Builder, 0, len(b))
			for builderName, builder := range b {
				builder.Name = builderName
				builder.Master = master
				builderList = append(builderList, builder)
			}
			builders[master] = builderList
		}(m)
	}
	wg.Wait()
	if len(errs) > 0 {
		errString := "Failed to get retrieve builders:"
		for _, err := range errs {
			errString += fmt.Sprintf("\n%v", err)
		}
		return nil, fmt.Errorf(errString)
	}
	rv := map[string]*Builder{}
	for _, buildersForMaster := range builders {
		for _, b := range buildersForMaster {
			rv[b.Name] = b
		}
	}
	return rv, nil
}

// GetBuildSlaves returns a map whose keys are master names and values are
// sub-maps whose keys are slave names and values are BuildSlave objects.
func GetBuildSlaves() (map[string]map[string]*BuildSlave, error) {
	var mtx sync.Mutex
	res := map[string]map[string]*BuildSlave{}
	errs := map[string]error{}
	var wg sync.WaitGroup
	for _, master := range MASTER_NAMES {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			slaves := map[string]*BuildSlave{}
			err := get(BUILDBOT_URL+m+"/json/slaves", &slaves)
			mtx.Lock()
			defer mtx.Unlock()
			if err != nil {
				errs[m] = fmt.Errorf("Failed to retrieve buildslaves for %s: %s", m, err)
				return
			}
			for name, s := range slaves {
				s.Name = name
				s.Master = m
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
func getUningestedBuilds(db DB, m string) (map[string][]int, error) {
	defer metrics2.FuncTimer().Stop()
	// Get the latest and last-processed builds for all builders.
	latest, err := getLatestBuilds(m)
	if err != nil {
		return nil, fmt.Errorf("Failed to get latest builds: %s", err)
	}
	lastProcessed, err := db.GetLastProcessedBuilds(m)
	if err != nil {
		return nil, fmt.Errorf("Failed to get last-processed builds: %s", err)
	}
	// Find the range of uningested builds for each builder.
	type numRange struct {
		Start int // The last-ingested build number.
		End   int // The latest build number.
	}
	ranges := map[string]*numRange{}
	for _, id := range lastProcessed {
		b, err := db.GetBuild(id)
		if err != nil {
			return nil, err
		}
		ranges[b.Builder] = &numRange{
			Start: b.Number,
			End:   b.Number,
		}
	}
	for b, n := range latest {
		if _, ok := ranges[b]; !ok {
			ranges[b] = &numRange{
				Start: -1,
				End:   n,
			}
		} else {
			ranges[b].End = n
		}
	}
	// Create a slice of build numbers for the uningested builds.
	unprocessed := map[string][]int{}
	for b, r := range ranges {
		if r.End < r.Start {
			glog.Warningf("Cannot create slice of builds to ingest for %q; invalid range (%d, %d)", b, r.Start, r.End)
			continue
		}
		builds := make([]int, r.End-r.Start)
		for i := r.Start + 1; i <= r.End; i++ {
			builds[i-r.Start-1] = i
		}
		if len(builds) > 0 {
			unprocessed[b] = builds
		}
	}
	return unprocessed, nil
}

// ingestNewBuilds finds the set of uningested builds and ingests them.
func ingestNewBuilds(db DB, m string, repos *gitinfo.RepoMap) error {
	defer metrics2.FuncTimer().Stop()
	glog.Infof("Ingesting builds for %s", m)
	// TODO(borenet): Investigate the use of channels here. We should be
	// able to start ingesting builds as the data becomes available rather
	// than waiting until the end.
	buildsToProcess, err := getUningestedBuilds(db, m)
	if err != nil {
		return fmt.Errorf("Failed to obtain the set of uningested builds: %s", err)
	}
	unfinished, err := db.GetUnfinishedBuilds(m)
	if err != nil {
		return fmt.Errorf("Failed to obtain the set of unfinished builds: %s", err)
	}
	for _, b := range unfinished {
		if _, ok := buildsToProcess[b.Builder]; !ok {
			buildsToProcess[b.Builder] = []int{}
		}
		buildsToProcess[b.Builder] = append(buildsToProcess[b.Builder], b.Number)
	}

	// TODO(borenet): Can we ingest builders in parallel?
	errs := map[string]error{}
	for b, w := range buildsToProcess {
		for _, n := range w {
			if BUILD_BLACKLIST[b][n] {
				glog.Warningf("Skipping blacklisted build: %s # %d", b, n)
				continue
			}
			if IsTrybot(b) {
				continue
			}
			glog.Infof("Ingesting build: %s, %s, %d", m, b, n)
			build, err := retryGetBuildFromMaster(m, b, n, repos)
			if err != nil {
				// If we couldn't get the build from the master after multiple
				// tries, assume that the build has somehow disappeared and
				// skip it.
				glog.Errorf("Failed to retrieve build from master; skipping: %s", err)
				continue
			}
			if err := IngestBuild(db, build, repos); err != nil {
				errs[b] = fmt.Errorf("Failed to ingest build: %s", err)
				break
			}
		}
	}
	if len(errs) > 0 {
		msg := fmt.Sprintf("Encountered errors ingesting builds for %s:", m)
		for b, err := range errs {
			msg += fmt.Sprintf("\n%s: %s", b, err)
		}
		return fmt.Errorf(msg)
	}
	glog.Infof("Done ingesting builds for %s", m)
	return nil
}

// NumTotalBuilds finds the total number of builds which have ever run.
func NumTotalBuilds() (int, error) {
	total := 0
	for _, m := range MASTER_NAMES {
		latest, err := getLatestBuilds(m)
		if err != nil {
			return 0, fmt.Errorf("Failed to get latest builds: %s", err)
		}
		for _, n := range latest {
			total += n + 1 // Include build #0.
		}
	}
	return total, nil
}

// IngestNewBuildsLoop continually ingests new builds.
func IngestNewBuildsLoop(db DB, workdir string) error {
	local, ok := db.(*localDB)
	if !ok {
		return fmt.Errorf("Can only ingest builds with a local DB instance.")
	}
	cache := newIngestCache(local)
	repos := gitinfo.NewRepoMap(workdir)
	go func() {
		var wg sync.WaitGroup
		for _, m := range MASTER_NAMES {
			go func(master string) {
				defer wg.Done()
				lv := metrics2.NewLiveness("buildbot-ingest", map[string]string{"master": master})
				for _ = range time.Tick(10 * time.Second) {
					if err := ingestNewBuilds(cache, master, repos); err != nil {
						glog.Errorf("Failed to ingest new builds: %s", err)
					} else {
						lv.Reset()
					}
				}
			}(m)
		}
		wg.Wait()
	}()
	return nil
}
