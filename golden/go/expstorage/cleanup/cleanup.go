package cleanup

import (
	"context"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// Start begins a go routine that will repeat every 24 hours until the context is cancelled. On
// that cycle, it will update the expectations in firestore that are "in use", which is to say,
// the grouping+digest they represent were observed in the last N commits (the size of the sliding
// window or "tile"). Then, it deletes any expectations that fall outside the policy
func Start(ctx context.Context, ixr *indexer.Indexer, cleaner expstorage.Cleaner, exp expectations.Classifier, posMax, negMax time.Duration) error {
	if posMax < 0 || negMax < 0 {
		return skerr.Fmt("Not cleaning because a negative duration was provided.")
	}
	go func() {
		util.RepeatCtx(24*time.Hour, ctx, func(ctx context.Context) {
			sklog.Infof("Begin expectations clean cycle")
			defer metrics2.NewTimer("gold_cleanup_expectations").Stop()
			now := time.Now()
			idx := ixr.GetIndex()
			if err := update(ctx, idx, cleaner, exp, now); err != nil {
				sklog.Errorf("Error updating expectations during clean cycle: %s", err)
				return
			}
			if err := cleanup(ctx, cleaner, posMax, negMax, now); err != nil {
				sklog.Errorf("Error cleaning expectations: %s", err)
				return
			}
			sklog.Infof("Expectations clean cycle success")
		})
	}()
	return nil
}

// update identifies all triaged digests in the last N commits and uses the provided cleaner to
// mark those grouping/digest pairs as used.
func update(ctx context.Context, idx indexer.IndexSearcher, cleaner expstorage.Cleaner, classifier expectations.Classifier, now time.Time) error {
	byTest := idx.DigestCountsByTest(types.IncludeIgnoredTraces)
	var allExp []expstorage.Delta
	for tn, dc := range byTest {
		for digest := range dc {
			// Untriaged digests will not (usually) be in the DB, so we shouldn't try to
			// update them.
			if classifier.Classification(tn, digest) == expectations.Untriaged {
				continue
			}
			allExp = append(allExp, expstorage.Delta{
				Grouping: tn,
				Digest:   digest,
			})
		}
	}
	if err := cleaner.SetUsed(ctx, allExp, now); err != nil {
		return skerr.Wrapf(err, "setting %d entries used at %s", len(allExp), now)
	}
	sklog.Infof("%d expectation entries touched", len(allExp))
	return nil
}

// cleanup removes positive and negative digests that
func cleanup(ctx context.Context, cleaner expstorage.Cleaner, posMax time.Duration, negMax time.Duration, now time.Time) error {
	if posMax > 0 {
		if n, err := cleaner.UntriageOldEntries(ctx, expectations.Positive, now.Add(-posMax)); err != nil {
			return skerr.Wrapf(err, "untriaging positive expectation entries before %s", now.Add(-posMax))
		} else {
			sklog.Infof("%d positive expectations have aged out", n)
		}
	}

	if negMax > 0 {
		if n, err := cleaner.UntriageOldEntries(ctx, expectations.Negative, now.Add(-negMax)); err != nil {
			return skerr.Wrapf(err, "untriaging negative expectation entries before %s", now.Add(-negMax))
		} else {
			sklog.Infof("%d negative expectations have aged out", n)
		}
	}
	// Clean out all untriaged expectations - they don't really need to be in the DB.
	if n, err := cleaner.PruneUntriagedEntries(ctx); err != nil {
		return skerr.Wrapf(err, "pruning untriaged expectation entries ")
	} else {
		sklog.Infof("%d untriaged expectations have aged out", n)
	}
	return nil
}
