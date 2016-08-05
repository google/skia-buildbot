package main

import (
	"fmt"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/blacklist"
	"go.skia.org/infra/build_scheduler/go/bot_map"
	"go.skia.org/infra/build_scheduler/go/build_queue"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
)

type buildslaveSlice []*buildbot.BuildSlave

func (s buildslaveSlice) Len() int           { return len(s) }
func (s buildslaveSlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s buildslaveSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// getFreeBuildslaves returns a slice of names of buildslaves which are free.
func getFreeBuildslaves() ([]*buildbot.BuildSlave, error) {
	errMsg := "Failed to get free buildslaves: %v"
	// Get the set of builders for each master.
	builders, err := buildbot.GetBuilders()
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}

	// Get the set of buildslaves for each master.
	slaves, err := buildbot.GetBuildSlaves()
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}
	// Flatten the buildslaves list.
	buildslaves := map[string]*buildbot.BuildSlave{}
	for _, slavesMap := range slaves {
		for slavename, slaveobj := range slavesMap {
			buildslaves[slavename] = slaveobj
		}
	}

	// Empty the buildslaves' Builders lists and only include builders not
	// in the blacklist.
	for _, bs := range buildslaves {
		bs.Builders = map[string][]int{}
	}

	// Map the builders to buildslaves.
	for _, b := range builders {
		// Only include builders for which there are no already-pending builds.
		if b.PendingBuilds == 0 {
			for _, slave := range b.Slaves {
				buildslaves[slave].Builders[b.Name] = nil
			}
		}
	}

	// Return the buildslaves which are connected and idle.
	// TODO(borenet): Include swarm trigger bots in this list.
	rv := []*buildbot.BuildSlave{}
	for _, s := range buildslaves {
		if len(s.RunningBuilds) == 0 && s.Connected {
			rv = append(rv, s)
		}
	}
	return rv, nil
}

// getFreeSwarmTriggers returns a map whose keys are free swarming trigger bot
// names and whose values are slices of builder names which those trigger bots
// may run.
func getFreeSwarmTriggers(s *swarming.ApiClient) (map[string][]string, error) {
	rv := map[string][]string{}
	bots, err := s.ListSkiaTriggerBots()
	if err != nil {
		return nil, err
	}
	for _, bot := range bots {
		if bot.IsDead {
			glog.Infof("Bot %s is dead! Skipping.", bot.BotId)
			continue
		}
		if bot.Quarantined {
			glog.Infof("Bot %s is quarantined! Skipping.", bot.BotId)
			continue
		}
		if bot.TaskId != "" {
			continue
		}
		rv[bot.BotId] = bot_map.BUILDERS_BY_SWARMING_BOT[bot.BotId]
	}
	return rv, nil
}

// BuildScheduler is a struct used for scheduling builds on bots.
type BuildScheduler struct {
	bb             *buildbucket.Client
	bl             *blacklist.Blacklist
	cachedBuilders []string
	cachedCommits  []string
	cacheMtx       sync.RWMutex
	local          bool
	q              *build_queue.BuildQueue
	repos          *gitinfo.RepoMap
	status         *BuildSchedulerStatus
	statusMtx      sync.RWMutex
}

// Builders returns the known list of builders.
func (bs *BuildScheduler) Builders() []string {
	bs.cacheMtx.RLock()
	defer bs.cacheMtx.RUnlock()
	return bs.cachedBuilders
}

// updateBuilders updates the known list of builders.
func (bs *BuildScheduler) updateBuilders() error {
	// TODO(borenet): Include SwarmBucket builders in this list.
	buildersMap, err := buildbot.GetBuilders()
	if err != nil {
		return fmt.Errorf("Failed to retrieve builders list: %s", err)
	}
	builders := make([]string, 0, len(buildersMap))
	for b, _ := range buildersMap {
		builders = append(builders, b)
	}
	sort.Strings(builders)
	bs.cacheMtx.Lock()
	defer bs.cacheMtx.Unlock()
	bs.cachedBuilders = builders
	return nil
}

// Commits returns a list of recent commits.
func (bs *BuildScheduler) Commits() []string {
	bs.cacheMtx.RLock()
	defer bs.cacheMtx.RUnlock()
	return bs.cachedCommits
}

// updateCommits updates the known set of commits.
func (bs *BuildScheduler) updateCommits() error {
	c := bs.q.RecentCommits()
	bs.cacheMtx.Lock()
	defer bs.cacheMtx.Unlock()
	bs.cachedCommits = c
	return nil
}

// Trigger forcibly triggers builds on the given builders at the given
// commit. It does not check whether there is a free buildslave to run the
// build.
func (bs *BuildScheduler) Trigger(builderNames []string, commit string) ([]*buildbucket.Build, error) {
	// Find the desired commit's repo and author.
	author := ""
	repoName := ""
	for _, r := range REPOS {
		repo, err := bs.repos.Repo(r)
		if err != nil {
			return nil, err
		}
		details, err := repo.Details(commit, false)
		if err == nil {
			author = details.Author
			repoName = r
			break
		}
	}
	if repoName == "" {
		return nil, fmt.Errorf("Unable to find commit %s in any repo.", commit)
	}

	// Find the desired builders.
	builders := make([]*buildbot.Builder, 0, len(builderNames))
	allBuilders, err := buildbot.GetBuilders()
	if err != nil {
		return nil, err
	}
	for _, builderName := range builderNames {
		b, ok := allBuilders[builderName]
		if !ok {
			return nil, fmt.Errorf("Unknown builder %s", builderName)
		}
		builders = append(builders, b)
	}

	// Schedule the build.
	rv := make([]*buildbucket.Build, 0, len(builders))
	for _, b := range builders {
		var scheduled *buildbucket.Build
		if bs.local {
			glog.Infof("Would schedule: %s @ %s", b.Name, commit)
			scheduled = &buildbucket.Build{
				Bucket: b.Master,
				Id:     fmt.Sprintf("ID:%s", b.Name),
			}
		} else {
			var err error
			scheduled, err = bs.bb.RequestBuild(b.Name, b.Master, commit, repoName, author)
			if err != nil {
				return nil, err
			}
		}
		rv = append(rv, scheduled)
	}
	return rv, nil
}

// scheduleBuilds finds builders with no pending builds, pops the
// highest-priority builds for each from the queue, and requests builds using
// buildbucket.
func (bs *BuildScheduler) scheduleBuilds() error {
	errMsg := "Failed to schedule builds: %v"

	// Get the list of idle buildslaves, update the BuildQueue.
	var wg sync.WaitGroup
	var free []*buildbot.BuildSlave
	var freeErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		free, freeErr = getFreeBuildslaves()
	}()

	var updateErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateErr = bs.q.Update()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := bs.updateBuilders(); err != nil {
			glog.Error(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := bs.updateCommits(); err != nil {
			glog.Error(err)
		}
	}()

	wg.Wait()
	if updateErr != nil {
		return fmt.Errorf(errMsg, updateErr)
	}
	if freeErr != nil {
		return fmt.Errorf(errMsg, freeErr)
	}

	// Sort the list of idle buildslaves, for saner log viewing.
	sort.Sort(buildslaveSlice(free))

	// Schedule builds on free buildslaves.
	allBuildersMap := map[string]bool{}
	errs := []error{}
	glog.Infof("Free buildslaves:")
	for _, s := range free {
		glog.Infof("\t%s", s.Name)
		builders := make([]string, 0, len(s.Builders))
		for b, _ := range s.Builders {
			builders = append(builders, b)
		}
		build, err := bs.q.Pop(builders)
		if err == build_queue.ERR_EMPTY_QUEUE {
			continue
		}
		if bs.local {
			glog.Infof("Would schedule: %s @ %s, score = %0.3f", build.Builder, build.Commit[0:7], build.Score)
		} else {
			scheduled, err := bs.bb.RequestBuild(build.Builder, s.Master, build.Commit, build.Repo, build.Author)
			if err != nil {
				errs = append(errs, err)
			} else {
				glog.Infof("Scheduled: %s @ %s, score = %0.3f, id = %s", build.Builder, build.Commit[0:7], build.Score, scheduled.Id)
			}
		}
		for _, builder := range builders {
			allBuildersMap[builder] = true
		}
	}

	if len(errs) > 0 {
		errString := "Got errors when scheduling builds:"
		for _, err := range errs {
			errString += fmt.Sprintf("\n%v", err)
		}
		return fmt.Errorf(errString)
	}
	bs.setStatus(&BuildSchedulerStatus{
		LastScheduled: time.Now(),
		TopCandidates: bs.q.TopN(10),
	})
	return nil
}

func StartNewBuildScheduler(period time.Duration, scoreThreshold, scoreDecay24Hr float64, db buildbot.DB, bb *buildbucket.Client, repos *gitinfo.RepoMap, workdir string, local bool) *BuildScheduler {
	// Build the queue.
	bl, err := blacklist.FromFile(path.Join(workdir, "blacklist.json"))
	if err != nil {
		glog.Fatal(err)
	}

	q, err := build_queue.NewBuildQueue(period, repos, scoreThreshold, scoreDecay24Hr, bl, db)
	if err != nil {
		glog.Fatal(err)
	}

	bs := &BuildScheduler{
		bb:             bb,
		bl:             bl,
		cachedBuilders: []string{},
		cachedCommits:  []string{},
		cacheMtx:       sync.RWMutex{},
		local:          local,
		q:              q,
		repos:          repos,
		status: &BuildSchedulerStatus{
			LastScheduled: time.Time{},
			TopCandidates: []*build_queue.BuildCandidate{},
		},
		statusMtx: sync.RWMutex{},
	}

	// Start scheduling builds in a loop.
	liveness := metrics2.NewLiveness("time-since-last-successful-scheduling", nil)
	if err := bs.scheduleBuilds(); err != nil {
		glog.Errorf("Failed to schedule builds: %v", err)
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			if err := bs.scheduleBuilds(); err != nil {
				glog.Errorf("Failed to schedule builds: %v", err)
			} else {
				liveness.Reset()
			}
		}
	}()
	return bs
}

func (bs *BuildScheduler) setStatus(status *BuildSchedulerStatus) {
	bs.statusMtx.Lock()
	defer bs.statusMtx.Unlock()
	bs.status = status
}

func (bs *BuildScheduler) Status() *BuildSchedulerStatus {
	bs.statusMtx.RLock()
	defer bs.statusMtx.RUnlock()
	return bs.status
}

type BuildSchedulerStatus struct {
	LastScheduled time.Time                     `json:"last_scheduled"`
	TopCandidates []*build_queue.BuildCandidate `json:"top_candidates"`
}

func (bs *BuildScheduler) GetBlacklist() *blacklist.Blacklist {
	return bs.bl
}
