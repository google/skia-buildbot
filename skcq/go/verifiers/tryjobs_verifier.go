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

func NewTryJobsVerifier(httpClient *http.Client) (Verifier, error) {

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

	return FailureState, fmt.Sprintf("%s This CL somethingsomethingsomething", av.Name()), nil
}

func (cv *TryJobsVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
}
