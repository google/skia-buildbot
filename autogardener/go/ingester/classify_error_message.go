package ingester

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/autogardener/go/utils"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
)

const (
	maxRecentFailureClasses           = 25
	ngramSize                         = 4
	ngramSimilarityCandidateThreshold = 0.10
	ngramFastPathThreshold            = 0.90
	ngramOverlapCandidateThreshold    = 0.50
	minNgramOverlapLength             = 25
	minStringContainsLength           = 30
	minClassIDPrefixLength            = 16
	genericFailureClassID             = "generic-failure"
)

var errDone = errors.New("done")

type errorClassifier struct {
	db     db.AutoGardenerDB
	gemini gemini.Client
}

func newErrorClassifier(db db.AutoGardenerDB, gemini gemini.Client) *errorClassifier {
	return &errorClassifier{
		db:     db,
		gemini: gemini,
	}
}

func (c *errorClassifier) classifyExtractedErrors(ctx context.Context, input <-chan *types.TaskAndError) {
	for {
		select {
		case job := <-input:
			if err := c.classifyExtractedError(ctx, job.Task, job.Error); err != nil {
				sklog.Errorf("Failed to classify extracted error: %s", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *errorClassifier) classifyExtractedError(ctx context.Context, task *ts_types.Task, extractedErr *types.ExtractedError) error {
	// Retrieve the ExtractedError from the DB to ensure that we haven't already
	// classified it (since the primary and backup queuing mechanisms may
	// overlap and enqueue the same TaskAndError multiple times).
	if exist, err := c.db.GetExtractedError(ctx, task.Id); err != nil {
		return skerr.Wrap(err)
	} else if exist.FailureClassId != "" {
		return nil
	}

	taskErrSanitized := utils.SanitizeErrorText(extractedErr.ErrorMessage)

	// Case 1: The error message is too generic. Skip classification entirely.
	var assignedFailureClass *types.FailureClass
	if utils.ErrorIsGeneric(extractedErr.ErrorMessage) || utils.ErrorIsGeneric(taskErrSanitized) {
		sklog.Debugf("Error message for %s is too generic.", task.Id)
		assignedFailureClass = makeGenericFailureClass()
		metrics2.GetFloat64SummaryMetric("autogardener_classification_similarity", map[string]string{"outcome": "generic"}).Observe(1.0)
	}

	// Case 2: Exact match of existing candidate.
	var failureClasses []*types.FailureClass
	if assignedFailureClass == nil {
		// TODO(borenet): We should probably do this per-repo and ensure
		// that we include the last N commits.
		windowStart := time.Now().Add(-90 * 24 * time.Hour)
		// TODO(borenet): These need to be cached, rather than hitting the DB for
		// every single failed task.
		var err error
		failureClasses, err = c.db.GetRecentFailureClasses(ctx, task.Repo, windowStart, 0)
		if err != nil {
			sklog.Errorf("Failed to retrieve recent failure classes: %s", err)
		}
		sklog.Debugf("Found %d recent failure classes", len(failureClasses))

		for _, cand := range failureClasses {
			candErr := cand.ErrorMessage()
			candErrSanitized := utils.SanitizeErrorText(candErr)
			if extractedErr.ErrorMessage == candErr || (taskErrSanitized != "" && taskErrSanitized == candErrSanitized) {
				sklog.Debugf("Exact match of error message for %s; no need to use gemini.", task.Id)
				metrics2.GetFloat64SummaryMetric("autogardener_classification_similarity", map[string]string{"outcome": "exact_match"}).Observe(1.0)
				assignedFailureClass = cand
				break
			}
		}
	}

	// Case 3: N-gram similarity and overlap.
	scores := make(map[*types.FailureClass]float64, len(failureClasses))
	var candidates []*types.FailureClass
	var bestScore float64
	if assignedFailureClass == nil {
		var bestCand *types.FailureClass
		for _, cand := range failureClasses {
			candErrSanitized := utils.SanitizeErrorText(cand.ErrorMessage())
			similarity := util.NgramSimilarity(taskErrSanitized, candErrSanitized, ngramSize)
			overlap := util.NgramOverlap(taskErrSanitized, candErrSanitized, ngramSize)

			minLen := len(taskErrSanitized)
			if len(candErrSanitized) < minLen {
				minLen = len(candErrSanitized)
			}

			// Include candidate if n-gram similarity or overlap is greater than the
			// thresholds, or one string exactly contains the other.
			isCandidate := (similarity >= ngramSimilarityCandidateThreshold) ||
				(minLen >= minNgramOverlapLength && overlap >= ngramOverlapCandidateThreshold) ||
				(minLen >= minStringContainsLength && (strings.Contains(candErrSanitized, taskErrSanitized) || strings.Contains(taskErrSanitized, candErrSanitized)))
			if !isCandidate {
				continue
			}

			candScore := max(similarity, overlap)
			scores[cand] = candScore
			candidates = append(candidates, cand)
			if similarity > bestScore {
				bestScore = similarity
				bestCand = cand
			}
		}
		if bestScore >= ngramFastPathThreshold {
			sklog.Debugf("N-gram fast-path match (similarity: %.2f) for task %s; assigning to %s.", bestScore, task.Id, bestCand.Id)
			metrics2.GetFloat64SummaryMetric("autogardener_classification_similarity", map[string]string{"outcome": "ngram_threshold_match"}).Observe(bestScore)
			assignedFailureClass = bestCand
		}
	}

	// Case 4: Match using Gemini.
	if assignedFailureClass == nil && len(candidates) > 0 {
		// Sort candidates in descending order of similarity
		sort.Slice(candidates, func(i, j int) bool {
			return scores[candidates[i]] > scores[candidates[j]]
		})

		err := util.ChunkIter(len(candidates), maxRecentFailureClasses, func(start, end int) error {
			classID, err := c.gemini.ClassifyFailure(ctx, extractedErr, candidates[start:end], task.Repo)
			if err != nil {
				return skerr.Wrap(err)
			}
			if classID != "" {
				for _, fc := range candidates[start:end] {
					if fc.Id == classID || (len(classID) >= minClassIDPrefixLength && strings.HasPrefix(fc.Id, classID)) {
						sklog.Infof("Gemini assigned task %s to failure class %s with N-gram similarity %.3f", task.Id, fc.Id, scores[fc])
						metrics2.GetFloat64SummaryMetric("autogardener_classification_similarity", map[string]string{"outcome": "gemini_match"}).Observe(scores[fc])
						assignedFailureClass = fc
						break
					}
				}
				if assignedFailureClass == nil {
					return skerr.Fmt("Failed to classify failure for task %s: unknown failure class with ID %q", task.Id, classID)
				}
				return errDone
			}
			return nil
		})
		if err != nil && skerr.Unwrap(err) != errDone {
			return skerr.Wrap(err)
		}
	}

	// Case 5: No match. Create a new FailureClass.
	if assignedFailureClass == nil {
		assignedFailureClass = &types.FailureClass{
			Id:   firestore.AlphaNumID(),
			Repo: task.Repo,
			Updates: []*types.FailureClassUpdate{
				{
					Timestamp:    now.Now(ctx),
					User:         "autogardener",
					ErrorMessage: types.Ptr(extractedErr.ErrorMessage),
					Analysis:     types.Ptr(extractedErr.Analysis),
				},
			},
		}
		sklog.Infof("Registered new FailureClass %s for task %s. Best similarity score was %.3f", assignedFailureClass.Id, task.Id, bestScore)
		metrics2.GetFloat64SummaryMetric("autogardener_classification_similarity", map[string]string{"outcome": "no_match"}).Observe(bestScore)
	}

	// Insert/update the FailureClass.
	if assignedFailureClass.LastSeen.Before(task.Finished) {
		assignedFailureClass.LastSeen = task.Finished
	}
	if err := c.db.PutFailureClass(ctx, assignedFailureClass); err != nil {
		return skerr.Wrapf(err, "failed to save failure class %s", assignedFailureClass.Id)
	}

	// Store the extracted error with the resolved FailureClassId.
	extractedErr.FailureClassId = assignedFailureClass.Id
	return skerr.Wrap(c.db.PutExtractedError(ctx, task.Id, extractedErr))
}

func makeGenericFailureClass() *types.FailureClass {
	return &types.FailureClass{
		Id: genericFailureClassID,
		Updates: []*types.FailureClassUpdate{
			{
				User:         "autogardener",
				ErrorMessage: types.Ptr(""),
				Analysis:     types.Ptr("The error did not contain enough identifiable details to classify successfully."),
			},
		},
	}
}
