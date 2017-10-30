package coverageingest

// The coverageingest package contains the code needed to download and interpret
// the results from our LLVM-based coverage tasks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
// produced by a list of jobs.  TODO(kjlubick): Add a combined view here (?) that
// shows a total coverage figure.
type IngestedResults struct {
	Commit *vcsinfo.ShortCommit `json:"info"`
	Jobs   []JobSummary         `json:"jobs"`
}

// JobSummary represents the parsed coverage data for a coverage Job.
type JobSummary struct {
	Name        string `json:"name"`
	TotalLines  int    `json:"lines"`
	MissedLines int    `json:"missed_lines"`
}

type JobSummarySlice []JobSummary

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

// The function parseSummary looks at a text summary file produced by a
// Coverage bot running with LLVM and extracts the JobSummary from it.
// It is a variable for easier mocking.
var parseSummary = defaultParseSummary

// This regex works for output from Clang/LLVM 5.0
var totalSummaryLine = regexp.MustCompile(`TOTAL\s+(?P<total_regions>\d+)\s+(?P<missed_regions>\d+)\s+(?P<regions_covered>[0-9\.]+%)\s+(?P<total_functions>\d+)\s+(?P<missed_functions>\d+)\s+(?P<functions_executed>[0-9\.]+%)\s+(?P<total_instantiations>\d+)\s+(?P<missed_instantiations>\d+)\s+(?P<instantiations_executed>[0-9\.]+%)\s+(?P<total_lines>\d+)\s+(?P<missed_lines>\d+)\s+(?P<lines_covered>[0-9\.]+%)`)

func defaultParseSummary(content string) JobSummary {
	if match := totalSummaryLine.FindStringSubmatch(content); match != nil {
		tl := match[indexForSubexpName("total_lines", totalSummaryLine)]
		ml := match[indexForSubexpName("missed_lines", totalSummaryLine)]
		return JobSummary{
			TotalLines:  util.SafeAtoi(tl),
			MissedLines: util.SafeAtoi(ml),
		}
	}
	sklog.Errorf("Could not parse summary from file: %s", content)
	return JobSummary{TotalLines: -1, MissedLines: -1}
}

// indexForSubexpName returns the index of a named regex subexpression. It's not
// complicated but reduces "magic numbers" and makes the logic of complicated
// regexes easier to follow.
func indexForSubexpName(name string, r *regexp.Regexp) int {
	return util.Index(name, r.SubexpNames())
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
		ingestedJobs := map[string]JobSummary{}
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
			if ext == "summary" {
				content, err := ioutil.ReadFile(outpath)
				if err != nil {
					sklog.Errorf("Problem reading summary file: %s", err)
					continue
				}
				js := parseSummary(string(content))
				js.Name = job
				ingestedJobs[job] = js
			}
		}
		jobs := JobSummarySlice{}
		for _, j := range ingestedJobs {
			jobs = append(jobs, j)
		}
		// Sort jobs alphabetically for determinism
		sort.Sort(jobs)
		newResults = append(newResults, IngestedResults{Commit: c.ShortCommit, Jobs: jobs})
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
		return nil, fmt.Errorf("Could not get ingestable files from path %s: %s", basePath, err)
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
func (s JobSummarySlice) Len() int           { return len(s) }
func (s JobSummarySlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s JobSummarySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
