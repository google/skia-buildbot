package cleanup

import (
	"context"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/types"
)

// Policy represents the configuration of how recently expectations need to be used before being
// cleaned up. If any duration is zero, digests of that type will not be cleaned up. Expectations
// are defined as being "used" when they show up in the recent sliding window of N commits.
type Policy struct {
	// PositiveMaxLastUsed is the oldest a Positive expectation will be kept around without being
	// used.
	PositiveMaxLastUsed time.Duration
	// NegativeMaxLastUsed is the oldest a Negative expectation will be kept around without being
	// used.
	NegativeMaxLastUsed time.Duration
}

// Validate returns an error if the policy is invalid.
func (p *Policy) Validate() error {
	if p.PositiveMaxLastUsed < 0 || p.NegativeMaxLastUsed < 0 {
		return skerr.Fmt("durations cannot be negative")
	}
	return nil
}

// Start begins a go routine that will repeat every 24 hours until the context is cancelled. On
// that cycle, it will update the expectations in Firestore that are "in use", which is to say,
// the grouping+digest they represent were observed in the last N commits (the size of the sliding
// window or "tile"). Then, it deletes any expectations that fall outside the policy provided.
func Start(ctx context.Context, ixr *indexer.Indexer, gc expectations.GarbageCollector, classifier expectations.Classifier, policy Policy) error {
	if err := policy.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	lastSuccess := metrics2.NewLiveness("last_expectations_cleanup")
	go func() {
		util.RepeatCtx(ctx, 24*time.Hour, func(ctx context.Context) {
			if err := ctx.Err(); err != nil {
				sklog.Warningf("context error: %s", err)
				return
			}
			sklog.Infof("Begin expectations clean cycle")
			defer metrics2.NewTimer("gold_cleanup_expectations").Stop()
			now := time.Now()
			idx := ixr.GetIndex()
			if err := update(ctx, idx.DigestCountsByTest(types.IncludeIgnoredTraces), gc, classifier, now); err != nil {
				sklog.Errorf("Error updating expectations during clean cycle: %s", err)
				return
			}
			if err := cleanup(ctx, gc, policy, now); err != nil {
				sklog.Errorf("Error cleaning expectations: %s", err)
				return
			}
			sklog.Infof("Expectations clean cycle success")
			lastSuccess.Reset()
		})
	}()
	return nil
}

// update identifies all triaged digests in the last N commits and uses the provided cleaner to
// mark those grouping/digest pairs as used.
func update(ctx context.Context, byTest map[types.TestName]digest_counter.DigestCount, gc expectations.GarbageCollector, classifier expectations.Classifier, now time.Time) error {
	var expToUpdate []expectations.ID
	for tn, dc := range byTest {
		for digest := range dc {
			// Untriaged digests will not (usually) be in the DB, so we shouldn't try to
			// update them.
			if classifier.Classification(tn, digest) == expectations.Untriaged {
				continue
			}
			expToUpdate = append(expToUpdate, expectations.ID{
				Grouping: tn,
				Digest:   digest,
			})
		}
	}
	if len(expToUpdate) == 0 {
		sklog.Infof("No triaged digests needed touching")
		return nil
	}
	if err := gc.UpdateLastUsed(ctx, expToUpdate, now); err != nil {
		return skerr.Wrapf(err, "setting %d entries used at %s", len(expToUpdate), now)
	}
	sklog.Infof("%d expectation entries touched", len(expToUpdate))
	return nil
}

// cleanup marks old positive and negative digests as untriaged and then deletes (prunes) all
// untriaged digests. It uses the provided durations as the threshold for cleanup.
func cleanup(ctx context.Context, gc expectations.GarbageCollector, policy Policy, now time.Time) error {
	posMax := policy.PositiveMaxLastUsed
	if posMax > 0 {
		if n, err := gc.MarkUnusedEntriesForGC(ctx, expectations.Positive, now.Add(-posMax)); err != nil {
			return skerr.Wrapf(err, "untriaging positive expectation entries before %s", now.Add(-posMax))
		} else {
			sklog.Infof("%d positive expectations have aged out", n)
		}
	}

	negMax := policy.NegativeMaxLastUsed
	if negMax > 0 {
		if n, err := gc.MarkUnusedEntriesForGC(ctx, expectations.Negative, now.Add(-negMax)); err != nil {
			return skerr.Wrapf(err, "untriaging negative expectation entries before %s", now.Add(-negMax))
		} else {
			sklog.Infof("%d negative expectations have aged out", n)
		}
	}
	// Clean out all untriaged expectations - they don't really need to be in the DB.
	if n, err := gc.GarbageCollect(ctx); err != nil {
		return skerr.Wrapf(err, "pruning untriaged expectation entries ")
	} else {
		sklog.Infof("%d untriaged expectations have aged out", n)
	}
	return nil
}
