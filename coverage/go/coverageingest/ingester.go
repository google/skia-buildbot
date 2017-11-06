package coverageingest

// The coverageingest package contains the code needed to download and interpret
// the results from our LLVM-based coverage tasks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

// Don't download the raw coverage data, which is put in a .tar.gz file for storage.
// We can't make anything out of it without the original binaries.
var INGEST_BLACKLIST = []*regexp.Regexp{regexp.MustCompile(`.+tar\.gz`)}

// The Ingester interface abstracts the logic for ingesting results from a source
// (e.g. GCS).  An Ingester should not be assumed to be thread safe.
type Ingester interface {
	// IngestCommits will ingest files belonging to the specified commits.
	IngestCommits([]*vcsinfo.LongCommit)

	// GetResults returns everything that was ingested on the last IngestCommits() call.
	GetResults() []IngestedResults
}

// The IngestedResults links information about a commit with the coverage information
// produced by a list of jobs.
type IngestedResults struct {
	Commit        *vcsinfo.ShortCommit `json:"info"`
	Jobs          []CoverageSummary    `json:"jobs"`
	TotalCoverage CoverageSummary      `json:"combined"`
}

// CoverageSummary represents the parsed coverage data for a coverage job.
type CoverageSummary struct {
	Name        string `json:"name"`
	TotalLines  int    `json:"lines"`
	MissedLines int    `json:"missed_lines"`
}

type CoverageSummarySlice []CoverageSummary

// The gcsingester implements the Ingester interface with Google Cloud Storage (GCS)
type gcsingester struct {
	dir       string
	gcsClient gcs.GCSClient
	results   []IngestedResults
}

// New returns an Ingester that is ready to be used.
func New(ingestionDir string, gcsClient gcs.GCSClient) *gcsingester {
	return &gcsingester{
		gcsClient: gcsClient,
		dir:       ingestionDir,
	}
}

// The function unTar will untar and unzip a .tar.gz file to a given output path.
// This tar file is assumed to be produced by our Coverage bots, which have
// a certain format (see defaultUnTar for details).
// It is a variable for easier mocking.
var unTar = defaultUnTar

func defaultUnTar(tarpath, outpath string) error {
	if _, err := fileutil.EnsureDirExists(outpath); err != nil {
		return fmt.Errorf("Could not set up directory to tar to: %s", err)
	}
	return exec.Run(&exec.Command{
		Name: "tar",
		// Strip components 6 removes /mnt/pd0/s/w/ir/coverage_html/ from the
		// tar file's internal folders.
		Args: []string{"xf", tarpath, "--strip-components=6", "-C", outpath},
	})
}

// indexForSubexpName returns the index of a named regex subexpression. It's not
// complicated but reduces "magic numbers" and makes the logic of complicated
// regexes easier to follow.
func indexForSubexpName(name string, r *regexp.Regexp) int {
	return util.Index(name, r.SubexpNames())
}

var calculateTotalCoverage = defaultCalculateTotalCoverage

func defaultCalculateTotalCoverage(folders ...string) (CoverageSummary, error) {
	if len(folders) == 0 {
		return CoverageSummary{}, nil
	}
	totalLines := 0
	missedLines := 0

	relPaths := util.StringSet{}

	// Make a list of all files in all folders
	for _, f := range folders {
		err := filepath.Walk(f, func(p string, info os.FileInfo, err error) error {
			if fi, err := os.Stat(p); err != nil {
				return fmt.Errorf("Could not get file info for %s: %s", p, err)
			} else if fi.IsDir() {
				return nil
			}
			relPath := strings.TrimPrefix(p, f)
			relPaths[relPath] = true
			return nil
		})
		if err != nil {
			sklog.Warningf("Possible error while walking directory %s: %s", f, err)
		}
	}

	for rp, _ := range relPaths {
		linesCovered := &coverageData{}
		for _, f := range folders {
			p := path.Join(f, rp)
			contents, err := ioutil.ReadFile(p)
			if err != nil {
				continue // file might not exist for all configurations
			}
			newlyCovered := parseLinesCovered(string(contents))
			linesCovered = linesCovered.Union(newlyCovered)
		}
		totalLines += linesCovered.Total()
		missedLines += linesCovered.Missed()
	}

	return CoverageSummary{TotalLines: totalLines, MissedLines: missedLines}, nil
}

// IngestCommits fulfills the Ingester interface.
func (n *gcsingester) IngestCommits(commits []*vcsinfo.LongCommit) {
	newResults := []IngestedResults{}
	for _, c := range commits {
		if _, err := fileutil.EnsureDirExists(path.Join(n.dir, c.Hash)); err != nil {
			sklog.Warningf("Could not create commit directories: %s", err)
		}

		basePath := "commit/" + c.Hash + "/"
		toDownload, err := n.getIngestableFilesFromGCS(basePath)
		if err != nil {
			sklog.Warningf("Problem ingesting for commit %s: %s", c, err)
			continue
		}
		toSummarize := map[string]string{}
	outer:
		for _, name := range toDownload {
			for _, b := range INGEST_BLACKLIST {
				if b.MatchString(name) {
					continue outer
				}
			}
			// There are at least 2 parts in the name. We expect something like:
			// Job.file
			// Job.type.tar
			parts := strings.Split(name, ".")
			outpath := path.Join(n.dir, c.Hash, name)
			if len(parts) == 1 {
				sklog.Warningf("Unknown file to ingest: %s", name)
				continue
			}
			// Don't re-download files that already exist
			if !fileExists(outpath) {
				if err := n.ingestFile(basePath, name, c.Hash); err != nil {
					sklog.Warningf("Problem ingesting file: %s", err)
					continue
				}
			}
			job := parts[0]
			ext := parts[1]
			if ext == "text" {
				// This is where the .text.tar gets extracted to.
				toSummarize[job] = path.Join(n.dir, c.Hash, job, ext, "coverage")
			}
		}
		jobs := CoverageSummarySlice{}
		toCombine := []string{}
		for job, folder := range toSummarize {
			cov, err := calculateTotalCoverage(folder)
			if err != nil {
				sklog.Warningf("Was unable to create a coverage data: %s", err)
				continue
			}
			cov.Name = job
			jobs = append(jobs, cov)
			if !strings.Contains(job, "CPU") {
				toCombine = append(toCombine, folder)
			}
		}
		// Sort jobs alphabetically for determinism
		sort.Sort(jobs)

		totalCoverage, err := calculateTotalCoverage(toCombine...)
		if err != nil {
			sklog.Warningf("Was unable to create a combined summary: %s", err)
		}
		newResults = append(newResults, IngestedResults{Commit: c.ShortCommit, Jobs: jobs, TotalCoverage: totalCoverage})
		sklog.Infof("Ingestion completed for commit %s - %s", c.ShortCommit.Hash, c.ShortCommit.Author)
	}

	n.results = newResults
}

// getIngestableFilesFromGCS returns the list of files to (possibly) ingest from GCS.
func (n *gcsingester) getIngestableFilesFromGCS(basePath string) ([]string, error) {
	toDownload := []string{}
	if err := n.gcsClient.AllFilesInDirectory(context.Background(), basePath, func(item *storage.ObjectAttrs) {
		name := strings.TrimPrefix(item.Name, basePath)
		toDownload = append(toDownload, name)
	}); err != nil {
		return nil, fmt.Errorf("Could not get ingestible files from path %s: %s", basePath, err)
	}
	return toDownload, nil
}

// ingestFile downloads the given file. If it is a tar file, it extracts it to a sub-folder
// based on the original file name.  E.g. My-Config.text.tar -> My-Config/text/
func (n *gcsingester) ingestFile(basePath, name, commit string) error {
	dl := basePath + name
	if contents, err := n.gcsClient.GetFileContents(context.Background(), dl); err != nil {
		return fmt.Errorf("Could not download file %s from GCS : %s", dl, err)

	} else {
		outpath := path.Join(n.dir, commit, name)
		file, err := os.Create(outpath)
		if err != nil {
			return fmt.Errorf("Could not open file %s for writing", outpath)
		}
		defer util.Close(file)
		if i, err := file.Write(contents); err != nil {
			return fmt.Errorf("Could not write completely to %s. Only wrote %d bytes: %s", outpath, i, err)
		}

		if strings.HasSuffix(name, "tar") {
			// Split My-Config-Name.type.tar into 3 parts.  type is "text" or "html"
			parts := strings.Split(name, ".")
			if len(parts) != 3 {
				return fmt.Errorf("Invalid tar name to ingest %s - must have 3 parts", name)
			}
			if err := unTar(outpath, path.Join(n.dir, commit, parts[0], parts[1])); err != nil {
				return fmt.Errorf("Could not untar %s: %s", outpath, err)
			}
		}
		return nil
	}
}

// GetResults fulfills the Ingester interface
func (n *gcsingester) GetResults() []IngestedResults {
	return n.results
}

// fileExists is a helper function that returns true if a file already exists at the given path.
func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else if err != nil {
		sklog.Warningf("Error getting file info about %s: %s", path, err)
		return false
	} else {
		return true
	}
}

// The following 3 lines implement sort.Interface
func (s CoverageSummarySlice) Len() int           { return len(s) }
func (s CoverageSummarySlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s CoverageSummarySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
