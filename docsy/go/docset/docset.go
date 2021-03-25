// docset keeps track of checkouts of a repository of Markdown documents.
package docset

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.skia.org/infra/docsy/go/codereview"
	"go.skia.org/infra/docsy/go/docsy"
	"go.skia.org/infra/go/git/gitinfo"
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

// DocSet represents a set of hugo rendered sets of documentation.
//
// The DocSet will run a background tasks that monitors issues and updates the
// patchsets as they progress, and removes issues as they are closed.
type DocSet interface {
	// FileSystem returns a FileSystem of the rendered contents of the
	// documentation. Pass in the invalidIssue to get the main branch.
	FileSystem(ctx context.Context, issue codereview.Issue) (http.FileSystem, error)

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
	docsy docsy.Docsy

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

	codeReview codereview.CodeReview

	// cache of rendered sets of documentation including the main set of docs
	// from HEAD at [InvalidIssue].
	cache map[codereview.Issue]*entry
}

func New(ctx context.Context, local bool, workDir string, docPath string, docsyDir string, repoURL string, codeReview codereview.CodeReview, docsy docsy.Docsy) (*docSet, error) {
	ret := &docSet{
		codeReview: codeReview,
		docPath:    docPath,
		docsyDir:   docsyDir,
		workDir:    workDir,
		repoURL:    repoURL,
		docsy:      docsy,
		cache:      map[codereview.Issue]*entry{},
	}
	// Don't return until we're ready to serve the main documents at HEAD.
	if _, err := ret.FileSystem(ctx, codeReview.MainIssue()); err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// FileSystem implements DocSet.
func (d *docSet) FileSystem(ctx context.Context, issue codereview.Issue) (http.FileSystem, error) {
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

func (d *docSet) refresh(ctx context.Context, issue codereview.Issue, patchsetRef string) (http.FileSystem, error) {
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

func (d *docSet) copyAndPatch(ctx context.Context, issue codereview.Issue, patchsetRef, srcDir string) error {
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

func (d *docSet) getCacheEntry(issue codereview.Issue) *entry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	if entry, ok := d.cache[issue]; ok {
		return entry
	}
	return nil
}

func (d *docSet) setCacheEntry(issue codereview.Issue, e *entry) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.cache[issue] = e
}

func (d *docSet) delCacheEntry(issue codereview.Issue) {
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
