package checkout_flags

import (
	"errors"
	"flag"

	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	GerritUrl  = flag.String("gerrit_url", "", "URL of the Gerrit instance.")
	PatchIssue = flag.String("patch_issue", "", "Issue ID, required if this is a try job.")
	PatchSet   = flag.String("patch_set", "", "Patch Set ID, required if this is a try job.")
	Repo       = flag.String("repo", "", "URL of the repo.")
	Revision   = flag.String("revision", "", "Git revision to check out.")
)

func GetRepoState() (types.RepoState, error) {
	var rs types.RepoState
	if *GerritUrl == "" {
		return rs, errors.New("--gerrit_url is required.")
	}
	if *Repo == "" {
		return rs, errors.New("--repo is required.")
	}
	if *Revision == "" {
		return rs, errors.New("--revision is required.")
	}
	rs.Repo = *Repo
	rs.Revision = *Revision
	if *PatchIssue != "" {
		rs.Patch = types.Patch{
			Issue:     *PatchIssue,
			PatchRepo: *Repo,
			Patchset:  *PatchSet,
			Server:    *GerritUrl,
		}
	}
	if !rs.Valid() {
		return rs, errors.New("RepoState is invalid.")
	}
	return rs, nil
}
