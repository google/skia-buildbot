// docset keeps track of checkouts of a repository of Markdown documents.
package docset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// refreshDuration is how often an issue cache entry should be checked to see if it's stale.
	refreshDuration = 10 * time.Minute

	// mainRefreshDuration is how often the main branch is updated.
	mainRefreshDuration = 5 * time.Minute
)

var (
	IssueClosedErr = errors.New("The requested issue has been merged or abandoned.")

	// timeNow is a function that returns the current time, for easy testing.
	timeNow = time.Now
)

// Docsy take an input directory and renders HTML/CSS into the destination directory.
type Docsy interface {
	Render(ctx context.Context, src, dst string) error
}

// docsy implements Docsy.
type docsy struct {
	// Absolute path the 'hugo' executable.
	hugoExe string

	// The directory where Docsy is located.
	docsyDir string

	// The relative path in the git repo where the docs are stored, e.g. "site"
	// for Skia.
	docPath string
}

func NewDocsy(hugoExe string, docsyDir string, docPath string) *docsy {
	return &docsy{
		hugoExe:  hugoExe,
		docsyDir: docsyDir,
		docPath:  docPath,
	}
}

// Render implements Docsy.
func (d *docsy) Render(ctx context.Context, src, dst string) error {
	cmd := executil.CommandContext(ctx,
		d.hugoExe,
		fmt.Sprintf("--source=%s", d.docsyDir),
		fmt.Sprintf("--destination=", dst),
		fmt.Sprintf("--config=", filepath.Join(d.docPath, "config.toml")),
		fmt.Sprintf("--contentDir=", src),
	)

	_, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = skerr.Wrapf(err, "adb reboot with stderr: %q", ee.Stderr)
		}
		return err
	}
	return nil
}

// Assert that docsy implements Docsy.
var _ Docsy = (*docsy)(nil)

// Issue is the identifier for a CodeReview issue.
type Issue string

// ListModifiedFilesResult is the results from calling CodeReviewListModifiedFiles.
type ListModifiedFilesResult struct {
	// Filename relative to the root of the git repo.
	Filename string

	// Deleted is true if the file was deleted at the given patchset.
	Deleted bool
}

// CodeReview represents an abstraction of the information we want from a code
// review system such as Gerrit.
type CodeReview interface {
	// MainIssue returns an identifier used to signal the use of the main branch
	// and not an actual Issue.
	MainIssue() Issue

	// ListModifiedFiles returns a list of the modified files for the given
	// issue at the given Ref.
	ListModifiedFiles(ctx context.Context, issue Issue, ref string) ([]ListModifiedFilesResult, error)

	// GetFiles returns the contents of the given file at the given Ref as a byte slice.
	GetFile(ctx context.Context, filename string, ref string) ([]byte, error)

	// GetPatchsetInfo returns the most recent patchset Ref of the given issue and
	// also if the issue has been closed.
	GetPatchsetInfo(ctx context.Context, issue Issue) (string, bool, error)
}

// gerritCodeView implements CodeReview.
type gerritCodeReview struct {
	// Gerrit used to interact with the Gerrit system.
	gc *gerrit.Gerrit

	// gitiles is used to download file contents.
	gitiles *gitiles.Repo
}

// NewGerritCodeReview returns a new instance of gerritCodeReview.
//
// The gerritURL value would probably be gerrit.GerritSkiaURL.
func NewGerritCodeReview(local bool, gerritURL, gitilesURL string) (*gerritCodeReview, error) {
	ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	gc, err := gerrit.NewGerrit(gerritURL, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gerritCodeReview{
		gc:      gc,
		gitiles: gitiles.NewRepo(gitilesURL, client),
	}, nil
}

// MainIssue implements CodeReview.
func (cr *gerritCodeReview) MainIssue() Issue {
	return "-1"
}

// ListModifiedFiles implements CodeReview.
func (cr *gerritCodeReview) ListModifiedFiles(ctx context.Context, issue Issue, ref string) ([]ListModifiedFilesResult, error) {
	// Convert Ref to patch.
	patch := path.Base(ref)
	issueInt64, err := strconv.ParseInt(string(issue), 10, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ret := []ListModifiedFilesResult{}

	files, err := cr.gc.Files(ctx, issueInt64, patch)
	for filename, fileinfo := range files {
		if filename == "/COMMIT_MSG" {
			continue
		}
		ret = append(ret, ListModifiedFilesResult{
			Filename: filename,
			Deleted:  fileinfo.Status == "D",
		})
	}
	return ret, nil
}

// GetFile implements CodeReview.
func (cr *gerritCodeReview) GetFile(ctx context.Context, filename, ref string) ([]byte, error) {
	return cr.gitiles.ReadFileAtRef(ctx, filename, ref)
}

// GetPatchsetInfo implements CodeReview.
func (cr *gerritCodeReview) GetPatchsetInfo(ctx context.Context, issue Issue) (string, bool, error) {
	changeInfo, err := cr.gc.GetChange(ctx, string(issue))
	if err != nil {
		return "", false, skerr.Wrap(err)
	}
	return changeInfo.Patchsets[len(changeInfo.Patchsets)-1].Ref, changeInfo.IsClosed(), nil
}

// Assert that gerritCodeReview implements the CodeReview interface.
var _ CodeReview = (*gerritCodeReview)(nil)

// DocSet represents a set of hugo rendered sets of documentation.
//
// The DocSet will run a background tasks that monitors issues and updates the
// patchsets as they progress, and removes issues as they are closed.
type DocSet interface {
	// FileSystem returns a FileSystem of the rendered contents of the
	// documentation. Pass in the invalidIssue to get the main branch.
	FileSystem(ctx context.Context, issue Issue) (http.FileSystem, error)

	// Start the background processes.
	Start() error
}

const (
	contentSubDirectory     = "content"
	destinationSubDirectory = "destination"
)

// entry in the docSet FileSystem cache.
type entry struct {
	mutex sync.Mutex

	patchsetRef string

	// FileSystem of the rendered contents of the documentation.
	fs http.FileSystem

	// The last time we checked if patchset is the latest.
	lastPatchsetCheck time.Time
}

// docSet implements DocSet.
//
// This implementation of DocSet works around a `workDir` which is presumed to
// be a directory with enough space to handle many checkouts of the
// documentation. It has the following structure:
//
//  {workDir}
//    /content/{issue}/ - A patched checkout of the documentation in the
//      docPath of the respository, with /-1/ representing the main branch at HEAD.
//    /destination/{issue}/ - The docsy rendered version of /content/{issue}.
//
// The background processes will monitor all current issues and update them to
// more recent patchsets periodically and also remove both /content/{issue}/ and
// /destination/{issue}/ directories once an issue is closed.
type docSet struct {
	// docsy renders the input files into the full static web site.
	docsy Docsy

	// The relative path in the git repo where the docs are stored, e.g. "site"
	// for Skia.
	docPath string

	// The directory containing Docsy. See https://docsy.dev.
	docsyDir string

	// wordDir is the directory where both the raw and processed documentation
	// live.
	workDir string

	// The URL of the repo passed to 'git clone'.
	repoURL string

	// mutex protects access to cache.
	mutex sync.RWMutex

	codeReview CodeReview

	// cache of rendered sets of documentation including the main set of docs
	// from HEAD at [InvalidIssue].
	cache map[Issue]*entry
}

func New(ctx context.Context, local bool, workDir string, docPath string, docsyDir string, repoURL string, codeReview CodeReview, docsy Docsy) (*docSet, error) {
	ret := &docSet{
		codeReview: codeReview,
		docPath:    docPath,
		docsyDir:   docsyDir,
		workDir:    workDir,
		repoURL:    repoURL,
		docsy:      docsy,
		cache:      map[Issue]*entry{},
	}
	// Don't return until we're ready to serve the main documents at HEAD.
	if _, err := ret.FileSystem(ctx, codeReview.MainIssue()); err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// FileSystem implements DocSet.
func (d *docSet) FileSystem(ctx context.Context, issue Issue) (http.FileSystem, error) {
	e := d.getCacheEntry(issue)
	now := timeNow()

	// If there is no cache entry, or the entry is too old, check if it's still
	// valid, and refresh the cache entry.
	if e == nil || now.After(e.lastPatchsetCheck.Add(refreshDuration)) {
		patchsetRef := ""
		isClosed := false
		if issue != d.codeReview.MainIssue() {
			var err error
			patchsetRef, isClosed, err = d.codeReview.GetPatchsetInfo(ctx, issue)
			if err != nil {
				if e == nil {
					return nil, skerr.Wrap(err)
				}
				sklog.Errorf("Failed to query issue ", err)
				return e.fs, nil
			}
		}
		if isClosed {
			d.delCacheEntry(issue)
			return nil, IssueClosedErr
		}
		if patchsetRef == e.patchsetRef {
			// Not updated, so just update the timestamp.
			e.lastPatchsetCheck = now
			d.setCacheEntry(issue, e)

			return e.fs, nil
		}
		return d.refresh(ctx, issue, patchsetRef)
	}

	return e.fs, nil
}

func (d *docSet) refresh(ctx context.Context, issue Issue, patchsetRef string) (http.FileSystem, error) {
	// First either clone the repo, in the case of issue == MainIssue, or copy
	// the docPath from MainIssue into /content/{issue}/ and then patch using
	// CodeReview.
	srcDir := filepath.Join(d.workDir, contentSubDirectory, string(issue), d.docPath)
	if issue == d.codeReview.MainIssue() {
		if _, err := gitinfo.CloneOrUpdate(ctx, d.repoURL, filepath.Join(d.workDir, contentSubDirectory, string(issue)), false); err != nil {
			return nil, skerr.Wrap(err)
		}
	} else {
		if err := d.copyAndPatch(ctx, issue, patchsetRef, srcDir); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	// Then run Hugo/Docsy over the files to create the rendered tree.
	dstDir := filepath.Join(d.workDir, destinationSubDirectory, string(issue), d.docPath)
	if err := d.docsy.Render(ctx, srcDir, dstDir); err != nil {
		return nil, skerr.Wrap(err)
	}

	// The return an http.Dir on the Docsy output.
	ret := http.Dir(dstDir)
	d.setCacheEntry(issue, &entry{
		fs:                ret,
		lastPatchsetCheck: timeNow(),
		patchsetRef:       patchsetRef,
	})

	return ret, nil
}

func (d *docSet) copyAndPatch(ctx context.Context, issue Issue, patchsetRef, srcDir string) error {
	// Copy files over from the main branch.
	mainSrcDir := filepath.Join(d.workDir, contentSubDirectory, string(d.codeReview.MainIssue()), d.docPath)
	if err := copyFilesAsLinks(mainSrcDir, srcDir); err != nil {
		return skerr.Wrap(err)
	}

	// Then download the patched files from the code review system.
	files, err := d.codeReview.ListModifiedFiles(ctx, issue, patchsetRef)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, fileinfo := range files {
		if fileinfo.Deleted {
			os.Remove(filepath.Join(srcDir, fileinfo.Filename))
			continue
		}

		b, err := d.codeReview.GetFile(ctx, fileinfo.Filename, patchsetRef)
		if err != nil {
			return skerr.Wrap(err)
		}
		err = util.WithWriteFile(filepath.Join(srcDir, fileinfo.Filename), func(w io.Writer) error {
			_, err := w.Write(b)
			if err != nil {
				return skerr.Wrap(err)
			}
			return nil
		})
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

func copyFilesAsLinks(src, dst string) error {
	// If this ends up being a bottleneck we can speed it up by launching a new
	// Go routine each time we hit a directory and handle the sub-dir in that Go
	// routine.
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrap(err)
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, path), 0755)
		}
		return os.Symlink(filepath.Join(src, path), filepath.Join(dst, path))
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func (d *docSet) getCacheEntry(issue Issue) *entry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	if entry, ok := d.cache[issue]; ok {
		return entry
	}
	return nil
}

func (d *docSet) setCacheEntry(issue Issue, e *entry) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.cache[issue] = e
}

func (d *docSet) delCacheEntry(issue Issue) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	delete(d.cache, issue)
}

// Start implements DocSet.
func (d *docSet) Start() error {
	go func() {
		ctx := context.Background()
		for range time.Tick(mainRefreshDuration) {
			d.refresh(ctx, d.codeReview.MainIssue(), "")
		}
	}()

	return nil
}

// Assert that docSet implements DocSet.
var _ DocSet = (*docSet)(nil)
