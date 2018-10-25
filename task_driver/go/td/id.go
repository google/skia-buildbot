package td

/*
   Creation of Task Driver ID is deterministic, based on properties of the run.
*/

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
)

func CreateID(rs db.RepoState, ts time.Time, seq uint64) string {
	// For non-tryjobs, the first component of the ID is the commit hash. We
	// expect that this will give us good distribution of IDs across
	// BigTable nodes.
	first := rs.Revision[:12]
	if rs.IsTryJob() {
		// First component matches GoB patch ref pattern ie. the last
		// two digits of the issue number, followed by the full issue
		// number, followed by the patchset. We expect that, since the
		// last two digits change relatively quickly, this will give us
		// decent distribution of IDs across BigTable nodes.
		first = fmt.Sprintf("%d-%d-%d", rs.GetShortIssue(), rs.Issue, rs.Patchset)
	}

	// The second component of the ID is the human-friendly repo identifier.
	second, ok := common.REPO_PROJECT_MAPPING[rs.Repo]
	if !ok {
		panic(fmt.Sprintf("Unknown repo URL: %s", rs.Repo))
	}

	// The third component of the ID is the timestamp.
	// TODO(borenet): We probably don't need a timestamp if we have a
	// sequence number provided by a central server.
	third := ts.UTC().Format(local_db.TIMESTAMP_FORMAT)

	// The fourth component is a sequence number.
	fourth := fmt.Sprintf(local_db.SEQUENCE_NUMBER_FORMAT, seq)

	return fmt.Sprintf("%s-%s-%s-%s", first, second, third, fourth)
}
