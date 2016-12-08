// poprepo is a package for populating a git repo with commits.
package poprepo

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
)

const (
	BUILDID_FILENAME = "BUILDID"
)

type PopRepoI interface {
	// GetLast returns the last committed buildid and its timestamp.
	GetLast() (string, int64, error)

	// Add a new buildid to the repo.
	Add(buildid string, ts int64) error
}

type PopRepo struct {
	checkout *git.Checkout
	workdir  string
}

func NewPopRepo(repoUrl, workdir string) (*PopRepo, error) {
	checkout, err := git.NewCheckout(repoUrl, workdir)
	if err != nil {
		return nil, fmt.Errorf("Unable to create the checkout of %q at %q: %s", repoUrl, workdir, err)
	}
	if err := checkout.Update(); err != nil {
		return nil, fmt.Errorf("Unable to update the checkout of %q at %q: %s", repoUrl, workdir, err)
	}
	return &PopRepo{
		checkout: checkout,
		workdir:  workdir,
	}, nil
}

func (p *PopRepo) GetLast() (string, int64, error) {
	fullpath := filepath.Join(p.workdir, BUILDID_FILENAME)
	b, err := ioutil.ReadFile(fullpath)
	if err != nil {
		return "", 0, fmt.Errorf("Unable to read file %q: %s", fullpath, err)
	}
	parts := strings.Split(string(b))
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("Unable to find just buildid and timestamp in: %q", string(b))
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("Timestamp is invalid in %q: %s", string(b), error)
	}
	return parts[0], ts, nil
}

/*

 # Write buildid and timestamp into the BUILDID file.

 # Convert API timestamp from ms to s. Use the buildid as the commit message.
 # Maybe make it a link?

 $ git add --all

 # Use GIT_COMMITTER_DATE and --date to ensure that the author and committer date agree.

 $ GIT_COMMITTER_DATE=1479855768 git commit -m "https://android-ingest.skia.org/r/3516196" --date=1479855768

 # To get the author timestamp just read it out of BUILDID.

 Note that the API provides multiple different commit timestamps, we're just going to pick
 the first, which is the earliest.

 # git push


 The unit tests call git init on an empty directory, then call Add and GetLast().
*/

func (p *PopRepo) Add(buildid string, ts int64) error {
	// Need to set GIT_COMMITTER_DATE with commit call.
	err := exec.RunCwd(string(g), append([]string{"git"}, cmd...)...)

	// Also needs to confirm that the buildids are ascending, which means they should be ints.
}
