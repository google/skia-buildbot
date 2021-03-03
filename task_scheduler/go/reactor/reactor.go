package reactor

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

type bot struct {
	ID         string
	Dimensions []string
}

type botDB interface {
	GetIdleBots() ([]*bot, error)
}

type candidateDB interface {
	AddCandidates([]*types.TaskCandidate) error
	// GetCandidates retrieves the TaskCandidates corresponding to the given
	// TaskKeys.  It returns the candidates in the same order as the respective
	// TaskKeys were given.  If any candidates did not exist for a particular
	// TaskKey, the candidate will be nil and no error is returned.
	GetCandidates(taskKeys []types.TaskKey) ([]*types.TaskCandidate, error)
	GetCandidatesInBlamelist(repo, revision, name string) ([]*types.TaskCandidate, error)
	GetTriggerableCandidates(dimensions []string) ([]*types.TaskCandidate, error)
	JobsWantExistingCandidate(taskKey types.TaskKey, jobID []string) error
	JobNoLongerWantsCandidates(taskKeys []types.TaskKey, jobID string) error
	TaskRunningForCandidate(taskKey types.TaskKey, taskID string) error
	UpdateBlamelists(candidates []*types.TaskCandidate) error
}

// Reactor manages scheduling based on external events.
type Reactor struct {
	db          db.DB
	botDB       botDB
	candidateDB candidateDB
}

// HandleJobsAdded handles the addition of new Jobs to the DB.
func (r *Reactor) HandleJobsAdded(jobs []*types.Job) error {
	// De-duplicate the candidates wanted by the new Jobs.
	jobIdsByTaskKey := make(map[types.TaskKey][]string, len(jobs))
	for _, job := range jobs {
		taskNames := make(map[string]bool, len(job.Dependencies))
		for name, deps := range job.Dependencies {
			taskNames[name] = true
			for _, dep := range deps {
				taskNames[dep] = true
			}
		}
		for taskName := range taskNames {
			forcedJobId := ""
			if job.IsForce {
				forcedJobId = job.Id
			}
			taskKey := types.TaskKey{
				RepoState:   job.RepoState,
				Name:        taskName,
				ForcedJobId: forcedJobId,
			}
			jobIdsByTaskKey[taskKey] = append(jobIdsByTaskKey[taskKey], job.Id)
		}
	}

	// Determine which candidates need to be added and which need to be updated.
	taskKeys := make([]types.TaskKey, 0, len(jobIdsByTaskKey))
	for taskKey := range jobIdsByTaskKey {
		taskKeys = append(taskKeys, taskKey)
	}
	candidates, err := r.candidateDB.GetCandidates(taskKeys)
	if err != nil {
		return skerr.Wrap(err)
	}
	updateCandidates := make([]*types.TaskCandidate, 0, len(candidates))
	newCandidates := make([]*types.TaskCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate != nil {
			updateCandidates = append(updateCandidates, candidate)
		} else {
			// TODO(borenet): Compute blamelist.
			candidate := &types.TaskCandidate{ /* TODO */ }
			r.computeBlamelist(candidate)
			newCandidates = append(newCandidates, candidate)
		}
	}

	// Add or update the candidates.
	if len(newCandidates) > 0 {
		if err := r.candidateDB.AddCandidates(newCandidates); err != nil {
			return skerr.Wrap(err)
		}
	}
	for _, candidate := range updateCandidates {
		jobIDs := jobIdsByTaskKey[candidate.TaskKey]
		if err := r.candidateDB.JobsWantExistingCandidate(candidate.TaskKey, jobIDs); err != nil {
			return skerr.Wrap(err)
		}
	}

	// We may have some idle bots which can run the new candidates.
	// TODO(borenet): We should probably also run this in a timed goroutine,
	// in case transient errors cause us to miss it.
	idleBots, err := r.botDB.GetIdleBots()
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, bot := range idleBots {
		if err := r.HandleBotAvailable(bot.ID, bot.Dimensions); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// HandleJobFinished handles the cancellation/finishing of a Job.
func (r *Reactor) HandleJobFinished(job *types.Job) error {
	taskKeys := make([]types.TaskKey, 0, len(job.Tasks))
	for taskName := range job.Dependencies {
		taskKey := types.TaskKey{
			RepoState: job.RepoState,
			Name:      taskName,
		}
		if job.IsForce {
			taskKey.ForcedJobId = job.Id
		}
		taskKeys = append(taskKeys, taskKey)
	}
	return r.candidateDB.JobNoLongerWantsCandidates(taskKeys, job.Id)
}

// HandleBotAvailable handles a newly-available bot.
func (r *Reactor) HandleBotAvailable(botID string, dimensions []string) error {
	// TODO(borenet): We need a lock for all candidates with overlapping
	// dimensions.

	// Find all of the TaskCandidates we could trigger for this bot, score them,
	// and choose the highest-scoring candidate.
	candidates, err := r.candidateDB.GetTriggerableCandidates(dimensions)
	if err != nil {
		return skerr.Wrap(err)
	}
	var topScore float64
	var topCandidate *types.TaskCandidate
	for _, c := range candidates {
		score := r.scoreCandidate(c)
		if score > topScore {
			topScore = score
			topCandidate = c
		}
	}
	if topCandidate == nil {
		return nil
	}

	// Trigger the task.
	taskID, err := r.triggerTask(topCandidate)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Update the TaskCandidate in the DB to indicate that we triggered a task
	// for it.
	if err := r.candidateDB.TaskRunningForCandidate(topCandidate.TaskKey, taskID); err != nil {
		return skerr.Wrap(err)
	}

	// Update the blamelists for the other candidates.
	candidates, err = r.candidateDB.GetCandidatesInBlamelist(topCandidate.Repo, topCandidate.Revision, topCandidate.Name)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.recomputeBlamelists(topCandidate, candidates)
	return skerr.Wrap(r.candidateDB.UpdateBlamelists(candidates))
}

// HandleTaskFinished handles a newly-finished task.
func (r *Reactor) HandleTaskFinished(task *types.Task) error {
	return skerr.Fmt("NOT IMPLEMENTED")
}

// computeBlamelist computes a blamelist for the given new TaskCandidate.
func (r *Reactor) computeBlamelist(candidate *types.TaskCandidate) {
	// TODO
}

// recomputeBlamelists updates the blamelists for the given TaskCandidates
// given the newly triggered TaskCandidate.
func (r *Reactor) recomputeBlamelists(triggered *types.TaskCandidate, candidates []*types.TaskCandidate) {
	// TODO
}

// scoreCandidate computes the score for a TaskCandidate.
func (r *Reactor) scoreCandidate(c *types.TaskCandidate) float64 {
	return 0 // TODO
}

// triggerTask triggers a Task for the given TaskCandidate.
func (r *Reactor) triggerTask(c *types.TaskCandidate) (string, error) {
	// TODO(borenet): Remember to update blamelists of existing Tasks.
	return "", skerr.Fmt("NOT IMPLEMENTED")
}
