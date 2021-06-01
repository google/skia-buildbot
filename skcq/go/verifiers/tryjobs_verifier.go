package verifiers

// Notes:
// Will have to add ability to trigger builds in go/buildbucket
// Also look at task_scheduler/go/tryjobs
// and buildbucket_util.py in cq

import (
	"fmt"
	"net/http"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
)

func NewTryJobsVerifier(httpClient *http.Client, change *gerrit.ChangeInfo) (Verifier, error) {

	// Get cqbots from tasks.json (or from the change...)

	return &TryJobsVerifier{
		bb2: buildbucket.NewClient(httpClient),
	}, nil
}

type TryJobsVerifier struct {
	// Maybe will not need this??
	// bb: *buildbucket_Api.Service

	bb2 buildbucket.BuildBucketInterface
}

func (av *TryJobsVerifier) Name() string {
	return "[TryJobsVerifier]"
}

func (av *TryJobsVerifier) Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {

	// Get list of all running try jobs in buildbucket - _fetch_buildset_try_jobs_v1 ?? in go/buildbucket/common there is GetTrybotsForCLPredicate
	// If they do not match the CQ list then manually trigger them.

	// Trigger them (using experimental stuff) and the keywords in the Footers!

	// Do not check for success/failure of experimental builds..

	return FailureState, fmt.Sprintf("%s This CL somethingsomethingsomething", av.Name()), nil
}

func (cv *TryJobsVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
}
