// Package docset keeps track of checkouts of a repository of Markdown documents
// and their rendered counterparts.
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

	"go.skia.org/infra/docsyserver/go/codereview"
	"go.skia.org/infra/docsyserver/go/docsy"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// refreshDuration is how often an issue cache entry should be checked to see if it's stale.
	refreshDuration = time.Minute

	// mainRefreshDuration is how often the main branch is updated.
	mainRefreshDuration = 5 * time.Minute
)

var (
	IssueClosedErr = errors.New("The requested issue has been merged or abandoned.")
)

// DocSet represents a set of hugo rendered documentation, one for the main
// branch, and then for any core review issues that contain documentation
// changes.
//
// The DocSet will run a background tasks that monitors issues and updates the
// patchsets as they progress, and removes issues as they are closed.
type DocSet interface {
	// FileSystem returns a FileSystem of the rendered contents of the
	// documentation. Pass in the codereview.MainIssue to get the main branch.
	FileSystem(ctx context.Context, issue codereview.Issue) (http.FileSystem, error)

	// Start the long running process that checks for updated issues and cleans
	// up closed issues.
	Start(ctx context.Context) error
}

const (
	// See the description of docSet for how these are used.
	contentSubDirectory     = "content"
	destinationSubDirectory = "destination"
)

// entry in the docSet FileSystem cache.
type entry struct {
	mutex sync.Mutex

	patchsetRef string

	// FileSystem of the rendered contents of the documentation.
	//
	// TODO(jcgregorio) It might be faster to make this an http.Handler.
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
//      docPath of the respository, with /main/ representing the main branch at HEAD.
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

	// codeReview allows querying info from the code review system.
	codeReview codereview.CodeReview

	// cache of rendered sets of documentation including the main set of docs
	// from HEAD at [InvalidIssue].
	cache map[codereview.Issue]*entry

	// A cache size metric.
	cacheSize metrics2.Int64Metric

	// Liveness for the refresher Go routine.
	liveness metrics2.Liveness
}

// New returns a new *docSet instance.
//
// wordDir is the directory where both the raw and processed documentation live.
//
// docPath is the relative path in the git repo where the docs are stored, e.g.
// "site" for Skia.
//
// docsyDir is the directory containing Docsy. See https://docsy.dev.
//
// repoURL is the URL of the repo passed to 'git clone'.
//
func New(workDir string, docPath string, docsyDir string, repoURL string, codeReview codereview.CodeReview, docsy docsy.Docsy) *docSet {
	ret := &docSet{
		codeReview: codeReview,
		docPath:    docPath,
		docsyDir:   docsyDir,
		workDir:    workDir,
		repoURL:    repoURL,
		docsy:      docsy,
		cache:      map[codereview.Issue]*entry{},
		cacheSize:  metrics2.GetInt64Metric("docsy_docset_cache_size"),
		liveness:   metrics2.NewLiveness("docsy_docset_refresh"),
	}

	return ret
}

// FileSystem implements DocSet.
func (d *docSet) FileSystem(ctx context.Context, issue codereview.Issue) (http.FileSystem, error) {
	e := d.getCacheEntry(issue)
	now := now.Now(ctx)

	// If there is no cache entry, or the entry is too old, check if it's still
	// valid, and refresh the cache entry.
	if e == nil || now.After(e.lastPatchsetCheck.Add(refreshDuration)) {
		// Default to values for MainIssue.
		patchsetRef := ""
		isClosed := false

		// Update patchsetRef and isClosed if we aren't on MainIsse.
		if issue != codereview.MainIssue {
			var err error
			patchsetRef, isClosed, err = d.codeReview.GetPatchsetInfo(ctx, issue)
			if err != nil {
				if e == nil {
					return nil, skerr.Wrap(err)
				}
				sklog.Errorf("Failed to query issue %s", err)
				// If we can't query the issue then just return the existing fs
				// even if it is old.
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

// refresh updates the files for the given issue at the given patchset.
func (d *docSet) refresh(ctx context.Context, issue codereview.Issue, patchsetRef string) (http.FileSystem, error) {
	sklog.Infof("Refreshing isue: %q patchset: %q", issue, patchsetRef)
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
		// Don't wrap so we get the hugo error output.
		return nil, err
	}

	// The return an http.Dir on the Docsy output.
	ret := http.Dir(dstDir)
	d.setCacheEntry(issue, &entry{
		fs:                ret,
		lastPatchsetCheck: now.Now(ctx),
		patchsetRef:       patchsetRef,
	})

	return ret, nil
}

// copyAndPatch copies over the documentation source from the main branch into a
// new directory as symlinks, and then overwrites any files changed in the issue
// with updated values fetched from the code review system.
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

// copyFilesAsLinks creates a copy of the directory 'src' in 'dst' using
// symlinks.
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
			if err := os.MkdirAll(filepath.Join(dst, relativePath), 0755); err != nil {
				return skerr.Wrapf(err, "Failed to create destination directory: %q", path)
			}
			return nil
		}
		srcFile := filepath.Join(src, relativePath)
		dstFile := filepath.Join(dst, relativePath)
		if fileutil.FileExists(dstFile) {
			if err := os.Remove(dstFile); err != nil {
				sklog.Warningf("Failed to remove %q: %s", dstFile, err)
			}
		}
		if err := os.Symlink(srcFile, dstFile); err != nil {
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
	contentDir := filepath.Join(d.workDir, contentSubDirectory, string(issue))
	destinationDir := filepath.Join(d.workDir, destinationSubDirectory, string(issue))
	if err := os.RemoveAll(contentDir); err != nil {
		sklog.Errorf("Failed to remove %q: %s", contentDir, err)
	}
	if err := os.RemoveAll(destinationDir); err != nil {
		sklog.Errorf("Failed to remove %q: %s", destinationDir, err)
	}
}

func (d *docSet) singleStep(ctx context.Context) error {
	for issue := range d.cache {
		if issue == codereview.MainIssue {
			continue
		}
		_, err := d.FileSystem(ctx, issue)
		if err != nil && err != IssueClosedErr {
			sklog.Errorf("Issue refresh failed for %q: %s", issue, err)
			continue
		}
	}

	// Always refresh MainIssue.
	_, err := d.refresh(ctx, codereview.MainIssue, "")
	if err != nil {
		return skerr.Wrapf(err, "DocSet refresh failed.")
	}
	d.liveness.Reset()
	return nil
}

// Start implements DocSet.
func (d *docSet) Start(ctx context.Context) error {
	if err := d.singleStep(ctx); err != nil {
		return skerr.Wrap(err)
	}

	ticker := time.NewTicker(mainRefreshDuration)
	done := ctx.Done()
	go func() {
		for {
			select {
			case <-done:
				sklog.Warning("Context cancelled")
				return
			case <-ticker.C:
				if err := d.singleStep(ctx); err != nil {
					sklog.Errorf("Failed single step in docSet background process: %s", err)
				}
			}
		}
	}()
	return nil
}

// Assert that docSet implements DocSet.
var _ DocSet = (*docSet)(nil)
