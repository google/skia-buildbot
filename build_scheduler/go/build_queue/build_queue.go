package build_queue

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
)

const (
	// Default score threshold for scheduling builds. This is "essentially zero",
	// allowing for significant floating point error, which indicates that we will
	// backfill builds for all commits except for those at which we've already built.
	DEFAULT_SCORE_THRESHOLD = 0.0001

	// Don't bisect builds with greater than this many commits. This
	// prevents spending lots of time computing giant blamelists.
	NO_BISECT_COMMIT_LIMIT = 100

	// If this time period used, include commits from the beginning of time.
	PERIOD_FOREVER = 0
)

var (
	// "Constants".

	// ERR_EMPTY_QUEUE is returned by BuildQueue.Pop() when the queue for
	// that builder is empty.
	ERR_EMPTY_QUEUE = fmt.Errorf("Queue is empty.")
)

// BuildCandidate is a struct which describes a candidate for a build.
type BuildCandidate struct {
	Author  string
	Commit  string
	Builder string
	Score   float64
	Repo    string
}

// BuildCandidateSlice is an alias to help sort BuildCandidates.
type BuildCandidateSlice []*BuildCandidate

func (s BuildCandidateSlice) Len() int { return len(s) }
func (s BuildCandidateSlice) Less(i, j int) bool {
	if s[i].Score == s[j].Score {
		// Fall back to sorting by commit hash to keep the sort order
		// consistent for testing.
		return s[i].Commit < s[j].Commit
	}
	return s[i].Score < s[j].Score
}
func (s BuildCandidateSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// BuildQueue is a struct which contains a priority queue for builders and
// commits.
type BuildQueue struct {
	botBlacklist   []*regexp.Regexp
	lock           sync.RWMutex
	period         time.Duration
	scoreThreshold float64
	queue          map[string][]*BuildCandidate
	repos          *gitinfo.RepoMap
	timeLambda     float64
}

// NewBuildQueue creates and returns a BuildQueue instance which considers
// commits in the specified time period.
//
// Build candidates with a score below the given scoreThreshold are not added
// to the queue. The score for a build candidate is defined as the value added
// by running that build, which is the difference between the total scores for
// all commits on a given builder before and after the build candidate would
// run. Scoring for an individual commit/builder pair is as follows:
//
// -1.0    if no build has ever included this commit on this builder.
// 1.0     if this builder has built AT this commit.
// 1.0 / N if a build on this builer has included this commit, where N is the
//         number of commits included in the build.
//
// The scoring works out such that build candidates which include commits which
// have never been included in a build have a value-add of >= 2, and other build
// candidates (eg. backfilling) have a value-add of < 2.
//
// Additionally, the scores include a time factor which serves to prioritize
// backfilling of more recent commits. The time factor is an exponential decay
// which is controlled by the timeDecay24Hr parameter. This parameter indicates
// what the time factor should be after 24 hours. For example, setting
// timeDecay24Hr equal to 0.5 causes the score for a build candidate with value
// 1.0 to be 0.5 if the commit is 24 hours old. At 48 hours with a value of 1.0
// the build candidate would receive a score of 0.25.
func NewBuildQueue(period time.Duration, repos *gitinfo.RepoMap, scoreThreshold, timeDecay24Hr float64, botBlacklist []*regexp.Regexp) (*BuildQueue, error) {
	if timeDecay24Hr <= 0.0 || timeDecay24Hr > 1.0 {
		return nil, fmt.Errorf("Time penalty must be 0 < p <= 1")
	}
	q := &BuildQueue{
		botBlacklist:   botBlacklist,
		lock:           sync.RWMutex{},
		period:         period,
		scoreThreshold: scoreThreshold,
		queue:          map[string][]*BuildCandidate{},
		repos:          repos,
		timeLambda:     lambda(timeDecay24Hr),
	}
	return q, nil
}

// lambda returns the lambda-value given a decay amount at 24 hours.
func lambda(decay float64) float64 {
	return (-math.Log(decay) / float64(24))
}

// timeFactor returns the time penalty factor, which is an exponential decay.
func timeFactor(now, t time.Time, lambda float64) float64 {
	hours := float64(now.Sub(t)) / float64(time.Hour)
	return math.Exp(-lambda * hours)
}

// scoreBuild returns the current score for the given commit/builder pair. The
// details on how scoring works are described in the doc for NewBuildQueue.
func scoreBuild(commit *gitinfo.LongCommit, build *buildbot.Build, now time.Time, timeLambda float64) float64 {
	s := -1.0
	if build != nil {
		if build.GotRevision == commit.Hash {
			s = 1.0
		} else if util.In(commit.Hash, build.Commits) {
			s = 1.0 / float64(len(build.Commits))
		}
	}
	return s * timeFactor(now, commit.Timestamp, timeLambda)
}

// Update retrieves the set of commits over a time period and the builds
// associated with those commits and builds a priority queue for commit/builder
// pairs.
func (q *BuildQueue) Update() error {
	return q.update(time.Now())
}

// update is the inner function which does all the work for Update. It accepts
// a time.Time so that time.Now() can be faked for testing.
func (q *BuildQueue) update(now time.Time) error {
	glog.Info("Updating build queue.")
	defer timer.New("BuildQueue.update()").Stop()
	queue := map[string][]*BuildCandidate{}
	errs := map[string]error{}
	mutex := sync.Mutex{}
	var wg sync.WaitGroup
	for _, repoUrl := range q.repos.Repos() {
		wg.Add(1)
		go func(repoUrl string) {
			defer wg.Done()
			candidates, err := q.updateRepo(repoUrl, now)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				errs[repoUrl] = err
				return
			}
			for k, v := range candidates {
				queue[k] = v
			}
		}(repoUrl)
	}
	wg.Wait()
	if len(errs) > 0 {
		msg := "Failed to update repos:"
		for repoUrl, err := range errs {
			msg += fmt.Sprintf("\n%s: %v", repoUrl, err)
		}
		return fmt.Errorf(msg)
	}

	// Update the queues.
	q.lock.Lock()
	defer q.lock.Unlock()
	q.queue = queue

	return nil
}

// updateRepo syncs the given repo and returns a set of BuildCandidates for
// each builder which uses it.
func (q *BuildQueue) updateRepo(repoUrl string, now time.Time) (map[string][]*BuildCandidate, error) {
	defer timer.New("BuildQueue.updateRepo()").Stop()
	errMsg := "Failed to update the repo: %v"

	// Sync/update the code.
	repo, err := q.repos.Repo(repoUrl)
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}
	if err := repo.Update(true, true); err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}

	// Get the details for all recent commits.
	from := now.Add(-q.period)
	if q.period == PERIOD_FOREVER {
		from = time.Unix(0, 0)
	}
	recentCommits := repo.From(from)

	// Pre-load commit details, from a larger window than we actually care about.
	fromPreload := now.Add(time.Duration(int64(-1.5 * float64(q.period))))
	if q.period == PERIOD_FOREVER {
		fromPreload = time.Unix(0, 0)
	}
	recentCommitsPreload := repo.From(fromPreload)
	for _, c := range recentCommitsPreload {
		if _, err := repo.Details(c); err != nil {
			return nil, fmt.Errorf(errMsg, err)
		}
	}

	// Get all builds associated with the recent commits.
	buildsByCommit, err := buildbot.GetBuildsForCommits(recentCommitsPreload, map[int]bool{})
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}

	// Create buildFinders for each builder.
	buildFinders := map[string]*buildFinder{}
	for _, buildsForCommit := range buildsByCommit {
		for _, build := range buildsForCommit {
			if util.AnyMatch(q.botBlacklist, build.Builder) {
				glog.Infof("Skipping blacklisted builder %s", build.Builder)
				continue
			}
			if _, ok := buildFinders[build.Builder]; !ok {
				bf, err := newBuildFinder(build.Builder, build.Master, build.Repository)
				if err != nil {
					return nil, fmt.Errorf(errMsg, err)
				}
				buildFinders[build.Builder] = bf
			}
			buildFinders[build.Builder].add(build)
		}
	}

	// Find candidates for each builder.
	candidates := map[string][]*BuildCandidate{}
	errs := map[string]error{}
	mutex := sync.Mutex{}
	var wg sync.WaitGroup
	for builder, finder := range buildFinders {
		wg.Add(1)
		go func(b string, bf *buildFinder) {
			defer wg.Done()
			c, err := q.getCandidatesForBuilder(bf, recentCommits, now)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				errs[b] = err
				return
			}
			candidates[b] = c
		}(builder, finder)
	}
	wg.Wait()
	if len(errs) > 0 {
		msg := "Failed to update the repo:"
		for _, err := range errs {
			msg += fmt.Sprintf("\n%v", err)
		}
		return nil, fmt.Errorf(msg)
	}
	return candidates, nil
}

// getCandidatesForBuilder finds all BuildCandidates for the given builder, in order.
func (q *BuildQueue) getCandidatesForBuilder(bf *buildFinder, recentCommits []string, now time.Time) ([]*BuildCandidate, error) {
	defer timer.New(fmt.Sprintf("getCandidatesForBuilder(%s)", bf.Builder)).Stop()
	repo, err := q.repos.Repo(bf.Repo)
	if err != nil {
		return nil, err
	}
	candidates := []*BuildCandidate{}
	for {
		score, newBuild, stoleFrom, err := q.getBestCandidate(bf, recentCommits, now)
		if err != nil {
			return nil, fmt.Errorf("Failed to get build candidates for %s: %v", bf.Builder, err)
		}
		if score < q.scoreThreshold {
			break
		}
		d, err := repo.Details(newBuild.GotRevision)
		if err != nil {
			return nil, err
		}
		// "insert" the new build.
		bf.add(newBuild)
		if stoleFrom != nil {
			bf.add(stoleFrom)
		}
		candidates = append(candidates, &BuildCandidate{
			Author:  d.Author,
			Builder: newBuild.Builder,
			Commit:  newBuild.GotRevision,
			Score:   score,
			Repo:    bf.Repo,
		})
	}
	return candidates, nil
}

// getBestCandidate finds the best BuildCandidate for the given builder.
func (q *BuildQueue) getBestCandidate(bf *buildFinder, recentCommits []string, now time.Time) (float64, *buildbot.Build, *buildbot.Build, error) {
	errMsg := fmt.Sprintf("Failed to get best candidate for %s: %%v", bf.Builder)
	repo, err := q.repos.Repo(bf.Repo)
	if err != nil {
		return 0.0, nil, nil, fmt.Errorf(errMsg, err)
	}
	// Find the current scores for each commit.
	currentScores := map[string]float64{}
	for _, commit := range recentCommits {
		currentBuild, err := bf.getBuildForCommit(commit)
		if err != nil {
			return 0.0, nil, nil, fmt.Errorf(errMsg, err)
		}
		d, err := repo.Details(commit)
		if err != nil {
			return 0.0, nil, nil, err
		}
		currentScores[commit] = scoreBuild(d, currentBuild, now, q.timeLambda)
	}

	// For each commit/builder pair, determine the score increase obtained
	// by running a build at that commit.
	scoreIncrease := map[string]float64{}
	newBuildsByCommit := map[string]*buildbot.Build{}
	stoleFromByCommit := map[string]*buildbot.Build{}
	for _, commit := range recentCommits {
		// Shortcut: Don't bisect builds with a huge number
		// of commits.  This saves lots of time and only affects
		// the first successful build for a bot.
		b, err := bf.getBuildForCommit(commit)
		if err != nil {
			return 0.0, nil, nil, fmt.Errorf(errMsg, err)
		}
		if b != nil {
			if len(b.Commits) > NO_BISECT_COMMIT_LIMIT {
				glog.Warningf("Skipping %s on %s; previous build has too many commits (#%d)", commit[0:7], b.Builder, b.Number)
				scoreIncrease[commit] = 0.0
				continue
			}
		}

		newScores := map[string]float64{}
		// Pretend to create a new Build at the given commit.
		newBuild := buildbot.Build{
			Builder:     bf.Builder,
			Master:      bf.Master,
			Number:      bf.MaxBuildNum + 1,
			GotRevision: commit,
			Repository:  bf.Repo,
		}
		commits, stealFrom, stolen, err := buildbot.FindCommitsForBuild(bf, &newBuild, q.repos)
		if err != nil {
			return 0.0, nil, nil, fmt.Errorf(errMsg, err)
		}
		// Re-score all commits in the new build.
		newBuild.Commits = commits
		for _, c := range commits {
			d, err := repo.Details(c)
			if err != nil {
				return 0.0, nil, nil, fmt.Errorf(errMsg, err)
			}
			if _, ok := currentScores[c]; !ok {
				// If this build has commits which are outside of our window,
				// insert them into currentScores to account for them.
				b, err := bf.getBuildForCommit(c)
				if err != nil {
					return 0.0, nil, nil, fmt.Errorf(errMsg, err)
				}
				score := scoreBuild(d, b, now, q.timeLambda)
				currentScores[c] = score
			}
			newScores[c] = scoreBuild(d, &newBuild, now, q.timeLambda)
		}
		newBuildsByCommit[commit] = &newBuild
		// If the new build includes commits previously included in
		// another build, update scores for commits in the build we stole
		// them from.
		if stealFrom != -1 {
			stoleFromOrig, err := bf.getByNumber(stealFrom)
			if err != nil {
				return 0.0, nil, nil, fmt.Errorf(errMsg, err)
			}
			if stoleFromOrig == nil {
				// The build may not be cached. Fall back on getting it from the DB.
				stoleFromOrig, err = buildbot.GetBuildFromDB(bf.Builder, bf.Master, stealFrom)
				if err != nil {
					return 0.0, nil, nil, fmt.Errorf(errMsg, err)
				}
				bf.add(stoleFromOrig)
			}
			// "copy" the build so that we can assign new commits to it
			// without modifying the cached build.
			stoleFromBuild := *stoleFromOrig
			newCommits := []string{}
			for _, c := range stoleFromBuild.Commits {
				if !util.In(c, stolen) {
					newCommits = append(newCommits, c)
				}
			}
			stoleFromBuild.Commits = newCommits
			for _, c := range stoleFromBuild.Commits {
				d, err := repo.Details(c)
				if err != nil {
					return 0.0, nil, nil, err
				}
				newScores[c] = scoreBuild(d, &stoleFromBuild, now, q.timeLambda)
			}
			stoleFromByCommit[commit] = &stoleFromBuild
		}
		// Sum the old and new scores.
		oldScoresList := make([]float64, 0, len(newScores))
		newScoresList := make([]float64, 0, len(newScores))
		for c, score := range newScores {
			oldScoresList = append(oldScoresList, currentScores[c])
			newScoresList = append(newScoresList, score)
		}
		oldTotal := util.Float64StableSum(oldScoresList)
		newTotal := util.Float64StableSum(newScoresList)
		scoreIncrease[commit] = newTotal - oldTotal
	}

	// Arrange the score increases by builder.
	candidates := []*BuildCandidate{}
	for commit, increase := range scoreIncrease {
		candidates = append(candidates, &BuildCandidate{
			Commit: commit,
			Score:  increase,
		})
	}
	sort.Sort(BuildCandidateSlice(candidates))
	best := candidates[len(candidates)-1]

	return best.Score, newBuildsByCommit[best.Commit], stoleFromByCommit[best.Commit], nil
}

// Pop retrieves the highest-priority item in the given set of builders and
// removes it from the queue. Returns nil if there are no items in the queue.
func (q *BuildQueue) Pop(builders []string) (*BuildCandidate, error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	var best *BuildCandidate
	for _, builder := range builders {
		s, ok := q.queue[builder]
		if !ok {
			// We don't yet know about this builder. In other words, it hasn't
			// built any commits. Therefore, the highest-priority commit to
			// build is tip-of-tree. Unfortunately, we don't know which repo
			// the bot uses, so we can only say "origin/master" and use the Skia
			// repo as a default.
			r, err := q.repos.Repo(q.repos.Repos()[0])
			if err != nil {
				return nil, err
			}
			h, err := r.FullHash("origin/master")
			if err != nil {
				return nil, err
			}
			details, err := r.Details(h)
			if err != nil {
				return nil, err
			}
			best = &BuildCandidate{
				Author:  details.Author,
				Builder: builder,
				Commit:  h,
				Repo:    q.repos.Repos()[0],
				Score:   math.MaxFloat64,
			}
			q.queue[builder] = []*BuildCandidate{best}
		} else {
			if len(s) > 0 {
				bc := s[0]
				if best == nil || bc.Score > best.Score {
					best = bc
				}
			}
		}
	}
	// Return the highest-priority commit for this builder.
	if best == nil {
		return nil, ERR_EMPTY_QUEUE
	}
	q.queue[best.Builder] = q.queue[best.Builder][1:len(q.queue[best.Builder])]
	return best, nil
}
