// docset keeps track of checkouts of a repository of Markdown documents.
package docset

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/docsy/go/codereview"
	"go.skia.org/infra/docsy/go/docsy"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/metrics2"
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
	Start()
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

	// A cache size metric.
	cacheSize metrics2.Int64Metric
}

func New(ctx context.Context, workDir string, docPath string, docsyDir string, repoURL string, codeReview codereview.CodeReview, docsy docsy.Docsy) (*docSet, error) {
	ret := &docSet{
		codeReview: codeReview,
		docPath:    docPath,
		docsyDir:   docsyDir,
		workDir:    workDir,
		repoURL:    repoURL,
		docsy:      docsy,
		cache:      map[codereview.Issue]*entry{},
		cacheSize:  metrics2.GetInt64Metric("docsy_docset_cache_size"),
	}
	// Don't return until we're ready to serve the main documents at HEAD.
	if _, err := ret.FileSystem(ctx, codereview.MainIssue); err != nil {
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
		if issue != codereview.MainIssue {
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
		if e != nil && patchsetRef == e.patchsetRef {
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
	if issue == codereview.MainIssue {
		if _, err := gitinfo.CloneOrUpdate(ctx, d.repoURL, filepath.Join(d.workDir, contentSubDirectory, string(issue)), false); err != nil {
			return nil, skerr.Wrap(err)
		}
	} else {
		srcDirWithoutDocPath := filepath.Join(d.workDir, contentSubDirectory, string(issue))
		if err := d.copyAndPatch(ctx, issue, patchsetRef, srcDirWithoutDocPath, d.docPath); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	// Then run Hugo/Docsy over the files to create the rendered tree.
	srcDir := filepath.Join(d.workDir, contentSubDirectory, string(issue), d.docPath)
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

func (d *docSet) copyAndPatch(ctx context.Context, issue codereview.Issue, patchsetRef, srcDirWithoutDocPath, docPath string) error {
	// Copy files over from the main branch.
	mainSrcDir := filepath.Join(d.workDir, contentSubDirectory, string(codereview.MainIssue), d.docPath)
	srcDir := filepath.Join(srcDirWithoutDocPath, docPath)
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
			_ = os.Remove(filepath.Join(srcDirWithoutDocPath, fileinfo.Filename))
			continue
		}

		// Check if file is a subdir of docPath.
		if !strings.HasPrefix(fileinfo.Filename, docPath) {
			continue
		}

		b, err := d.codeReview.GetFile(ctx, fileinfo.Filename, patchsetRef)
		if err != nil {
			return skerr.Wrap(err)
		}
		// srcDir contains the docPath, but fileinfo also contains the docPath.
		err = util.WithWriteFile(filepath.Join(srcDirWithoutDocPath, fileinfo.Filename), func(w io.Writer) error {
			_, err := w.Write(b)
			if err != nil {
				return skerr.Wrapf(err, "Failed to write: %q", fileinfo.Filename)
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
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrapf(err, "Could not descend into path: %q", path)
		}
		relativePath, err := filepath.Rel(src, path)
		if err != nil {
			return skerr.Wrapf(err, "Paths are not relative: %q %q", src, path)
		}
		if info.IsDir() {
			if err := os.MkdirAll(filepath.Join(dst, path), 0755); err != nil {
				return skerr.Wrapf(err, "Failed to create destination directory: %q", path)
			}
			return nil
		}
		if err := os.Symlink(filepath.Join(src, relativePath), filepath.Join(dst, relativePath)); err != nil {
			return skerr.Wrapf(err, "Failed to create symlink for %q", relativePath)
		}
		return nil
	})
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
	d.cacheSize.Update(int64(len(d.cache)))
}

func (d *docSet) delCacheEntry(issue codereview.Issue) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	delete(d.cache, issue)
	d.cacheSize.Update(int64(len(d.cache)))
}

// Start implements DocSet.
func (d *docSet) Start() {
	go func() {
		liveness := metrics2.NewLiveness("docsy_docset_refresh")
		ctx := context.Background()
		for range time.Tick(mainRefreshDuration) {
			_, err := d.refresh(ctx, codereview.MainIssue, "")
			if err != nil {
				sklog.Errorf("DocSet refresh failed: %s", err)
			} else {
				liveness.Reset()
			}
		}
	}()
}

// Assert that docSet implements DocSet.
var _ DocSet = (*docSet)(nil)
