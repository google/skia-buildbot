package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
)

func NewSubmittedTogetherVerifier(ctx context.Context, togetherChanges []*gerrit.ChangeInfo, httpClient *http.Client, skCQCfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, gitilesRepo *gitiles.Repo, configReader *config.GitilesConfigReader) (Verifier, error) {
	togetherChangesToVerifiers := map[*gerrit.ChangeInfo][]Verifier{}
	for _, tc := range togetherChanges {
		tcVerifiers, err := GetVerifiers(ctx, httpClient, skCQCfg, cr, tc, true /* isSubmittedTogetherChange */, gitilesRepo, configReader)
		if err != nil {
			return nil, skerr.Fmt("Could not get verifiers for the together change %d: %s", tc.Issue, err)
		}
		togetherChangesToVerifiers[tc] = tcVerifiers
	}
	return &SubmittedTogetherVerifier{
		togetherChangesToVerifiers: togetherChangesToVerifiers,
	}, nil
}

type SubmittedTogetherVerifier struct {
	togetherChangesToVerifiers map[*gerrit.ChangeInfo][]Verifier
}

func (stv *SubmittedTogetherVerifier) Name() string {
	return "[SubmittedTogetherVerifier]"
}

func (stv *SubmittedTogetherVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state VerifierState, reason string, err error) {
	successMsgsFromVerfiers := []string{}
	// Loop through all verifiers of all together changes and run them.
	for ts, tsVerifiers := range stv.togetherChangesToVerifiers {
		sm, wm, rm := RunVerifiers(ctx, ts, tsVerifiers, startTime)
		if len(rm) > 0 {
			return FailureState, fmt.Sprintf("%s Submitted together change https://skia-review.googlesource.com/c/%d has failed verifiers: %s", stv.Name(), ts.Issue, strings.Join(rm, "\n")), nil
		} else if len(wm) > 0 {
			return WaitingState, fmt.Sprintf("%s Submitted together change %d is waiting for verifiers: %s", stv.Name(), ts.Issue, strings.Join(wm, "\n")), nil
		} else {
			successMsgsFromVerfiers = append(successMsgsFromVerfiers, sm...)
		}
	}
	return SuccessState, fmt.Sprintf("%s Successfully ran all verifiers of submitted together change: %s", stv.Name(), strings.Join(successMsgsFromVerfiers, "\n")), nil
}

func (stv *SubmittedTogetherVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo) {
	// Loop through all verifiers of all together changes and run cleanup on them.
	for ts, tsVerifiers := range stv.togetherChangesToVerifiers {
		for _, tsVerifier := range tsVerifiers {
			tsVerifier.Cleanup(ctx, ts)
		}
	}
	return
}
