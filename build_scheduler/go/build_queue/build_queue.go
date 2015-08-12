package build_queue

import (
	"fmt"
	"math"
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

	// If this time period used, include commits from the beginning of time.
	PERIOD_FOREVER = 0
)

var (
	// "Constants".

	// ERR_EMPTY_QUEUE is returned by BuildQueue.Pop() when the queue for
	// that builder is empty.
	ERR_EMPTY_QUEUE = fmt.Errorf("Queue is empty.")

	// REPOS are the repositories to query.
	REPOS = []string{
		"https://skia.googlesource.com/skia.git",
		"https://skia.googlesource.com/buildbot.git",
	}
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
	botWhitelist   []string
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
func NewBuildQueue(period time.Duration, workdir string, scoreThreshold, timeDecay24Hr float64, botWhitelist []string) (*BuildQueue, error) {
	if timeDecay24Hr <= 0.0 || timeDecay24Hr > 1.0 {
		return nil, fmt.Errorf("Time penalty must be 0 < p <= 1")
	}
	q := &BuildQueue{
		botWhitelist:   botWhitelist,
		lock:           sync.RWMutex{},
		period:         period,
		scoreThreshold: scoreThreshold,
		queue:          map[string][]*BuildCandidate{},
		repos:          gitinfo.NewRepoMap(workdir),
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

	if err := q.repos.Update(); err != nil {
		return err
	}
	queue := map[string][]*BuildCandidate{}
	for _, repoUrl := range REPOS {
		repo, err := q.repos.Repo(repoUrl)
		if err != nil {
			return err
		}
		candidates, err := q.updateRepo(repoUrl, repo, now)
		if err != nil {
			return err
		}
		for k, v := range candidates {
			if _, ok := queue[k]; !ok {
				queue[k] = v
			} else {
				queue[k] = append(queue[k], v...)
			}
		}
	}

	// Sort the priorities.
	for _, prioritiesForBuilder := range queue {
		sort.Sort(BuildCandidateSlice(prioritiesForBuilder))
	}

	// Update the queues.
	q.lock.Lock()
	defer q.lock.Unlock()
	q.queue = queue

	return nil
}

// updateRepo syncs the given repo and returns a set of BuildCandidates for it.
func (q *BuildQueue) updateRepo(repoUrl string, repo *gitinfo.GitInfo, now time.Time) (map[string][]*BuildCandidate, error) {
	from := now.Add(-q.period)
	if q.period == PERIOD_FOREVER {
		from = time.Unix(0, 0)
	}
	recentCommits := repo.From(from)
	commitDetails := map[string]*gitinfo.LongCommit{}
	for _, c := range recentCommits {
		details, err := repo.Details(c)
		if err != nil {
			return nil, err
		}
		commitDetails[c] = details
	}

	// Get all builds associated with the recent commits.
	buildsByCommit, err := buildbot.GetBuildsForCommits(recentCommits, map[int]bool{})
	if err != nil {
		return nil, err
	}

	// Find the sets of all bots and masters, organize builds by
	// commit/builder and builder/number.
	masters := map[string]string{}
	builds := map[string]map[string]*buildbot.Build{}
	buildsByBuilderAndNum := map[string]map[int]*buildbot.Build{}
	for commit, buildsForCommit := range buildsByCommit {
		builds[commit] = map[string]*buildbot.Build{}
		for _, build := range buildsForCommit {
			if !util.In(build.Builder, q.botWhitelist) {
				continue
			}
			masters[build.Builder] = build.Master
			builds[commit][build.Builder] = build
			if _, ok := buildsByBuilderAndNum[build.Builder]; !ok {
				buildsByBuilderAndNum[build.Builder] = map[int]*buildbot.Build{}
			}
			buildsByBuilderAndNum[build.Builder][build.Number] = build
		}
	}
	allBots := make([]string, 0, len(masters))
	for builder, _ := range masters {
		allBots = append(allBots, builder)
	}

	// Find the current scores for each commit/builder pair.
	currentScores := map[string]map[string]float64{}
	for _, commit := range recentCommits {
		myBuilds, ok := builds[commit]
		if !ok {
			myBuilds = map[string]*buildbot.Build{}
		}
		currentScores[commit] = map[string]float64{}
		for _, builder := range allBots {
			currentScores[commit][builder] = scoreBuild(commitDetails[commit], myBuilds[builder], now, q.timeLambda)
		}
	}

	// For each commit/builder pair, determine the score increase obtained
	// by running a build at that commit.
	scoreIncrease := map[string]map[string]float64{}
	for _, commit := range recentCommits {
		scoreIncrease[commit] = map[string]float64{}
		for _, builder := range allBots {
			newScores := map[string]float64{}
			// Pretend to create a new Build at the given commit.
			newBuild := buildbot.Build{
				Builder:     builder,
				Master:      masters[builder],
				Number:      math.MaxInt32,
				GotRevision: commit,
				Repository:  repoUrl,
			}
			commits, stealFrom, stolen, err := buildbot.FindCommitsForBuild(&newBuild, q.repos)
			if err != nil {
				return nil, err
			}
			// Re-score all commits in the new build.
			newBuild.Commits = commits
			for _, c := range commits {
				if _, ok := currentScores[c]; !ok {
					// If this build has commits which are outside of our window,
					// insert them into currentScores to account for them.
					score := scoreBuild(commitDetails[commit], builds[commit][builder], now, q.timeLambda)
					currentScores[c] = map[string]float64{
						builder: score,
					}
				}
				if _, ok := commitDetails[c]; !ok {
					d, err := repo.Details(c)
					if err != nil {
						return nil, err
					}
					commitDetails[c] = d
				}
				newScores[c] = scoreBuild(commitDetails[c], &newBuild, now, q.timeLambda)
			}
			// If the new build includes commits previously included in
			// another build, update scores for commits in the build we stole
			// them from.
			if stealFrom != -1 {
				stoleFromOrig, ok := buildsByBuilderAndNum[builder][stealFrom]
				if !ok {
					// The build may not be cached. Fall back on getting it from the DB.
					stoleFromOrig, err = buildbot.GetBuildFromDB(builder, masters[builder], stealFrom)
					if err != nil {
						return nil, err
					}
					buildsByBuilderAndNum[builder][stealFrom] = stoleFromOrig
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
					newScores[c] = scoreBuild(commitDetails[c], &stoleFromBuild, now, q.timeLambda)
				}
			}
			// Sum the old and new scores.
			// First, sort the old and new scores to help with numerical stability.
			oldScoresList := make([]float64, 0, len(newScores))
			newScoresList := make([]float64, 0, len(newScores))
			for c, score := range newScores {
				oldScoresList = append(oldScoresList, currentScores[c][builder])
				newScoresList = append(newScoresList, score)
			}
			sort.Sort(sort.Float64Slice(oldScoresList))
			sort.Sort(sort.Float64Slice(newScoresList))
			oldTotal := 0.0
			newTotal := 0.0
			for i, _ := range oldScoresList {
				oldTotal += oldScoresList[i]
				newTotal += newScoresList[i]
			}
			scoreIncrease[commit][builder] = newTotal - oldTotal
		}
	}

	// Arrange the score increases by builder.
	candidates := map[string][]*BuildCandidate{}
	for commit, builders := range scoreIncrease {
		for builder, scoreIncrease := range builders {
			if _, ok := candidates[builder]; !ok {
				candidates[builder] = []*BuildCandidate{}
			}
			// Don't schedule builds below the given threshold.
			if scoreIncrease > q.scoreThreshold {
				candidates[builder] = append(candidates[builder], &BuildCandidate{
					Author:  commitDetails[commit].Author,
					Builder: builder,
					Commit:  commit,
					Repo:    repoUrl,
					Score:   scoreIncrease,
				})
			}
		}
	}

	return candidates, nil
}

// Pop retrieves the highest-priority item in the given set of builders and
// removes it from the queue. Returns nil if there are no items in the queue.
func (q *BuildQueue) Pop(builder string) (*BuildCandidate, error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	s, ok := q.queue[builder]
	if !ok {
		// We don't yet know about this builder. In other words, it hasn't
		// built any commits. Therefore, the highest-priority commit to
		// build is tip-of-tree. Unfortunately, we don't know which repo
		// the bot uses, so we can only say "origin/master", with no repo
		// or author values.
		return &BuildCandidate{
			Author:  "???",
			Builder: builder,
			Commit:  "origin/master",
			Repo:    "???",
			Score:   math.MaxFloat64,
		}, nil
	}
	if len(s) == 0 {
		// There are no commits that we still need to test on this builder.
		return nil, ERR_EMPTY_QUEUE
	}
	// Return the highest-priority commit for this builder.
	rv := s[len(s)-1]
	q.queue[builder] = q.queue[builder][0 : len(q.queue[builder])-1]
	return rv, nil
}
