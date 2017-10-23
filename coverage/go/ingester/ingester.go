package ingester

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var INGEST_BLACKLIST = []*regexp.Regexp{regexp.MustCompile(`.+tar\.gz`)}

type CoverageIngester interface {
	IngestCommits([]string)

	GetResults() []IngestedResults
}

type ingester struct {
	dir          string
	gcsClient    gcs.GCSClient
	results      []IngestedResults
	resultsMutex sync.Mutex
}

type JobSummary struct {
	Name        string `json:"name"`
	TotalLines  int    `json:"lines"`
	MissedLines int    `json:"missed_lines"`
}

type JobSummarySlice []JobSummary

type IngestedResults struct {
	Commit string       `json:"commit"`
	Jobs   []JobSummary `json:"jobs"`
}

func New(ingestionDir string, gcsClient gcs.GCSClient) *ingester {
	return &ingester{
		gcsClient: gcsClient,
		dir:       ingestionDir,
	}
}

// This is a variable for mocking.
var unTar = defaultUnTar

func defaultUnTar(tarpath, outpath string) error {
	if _, err := fileutil.EnsureDirExists(outpath); err != nil {
		return fmt.Errorf("Could not set up directory to tar to: %s", err)
	}
	return exec.Run(&exec.Command{
		Name: "tar",
		// Strip components 6 removes /mnt/pd0/s/w/ir/coverage_html/ from the
		// tar
		Args: []string{"xf", tarpath, "--strip-components=6", "-C", outpath},
	})
}

func (n *ingester) IngestCommits(commits []string) {
	newResults := []IngestedResults{}
	for _, c := range commits {
		if _, err := fileutil.EnsureDirExists(path.Join(n.dir, c)); err != nil {
			sklog.Warningf("Could not create commit directories: %s", err)
		}

		basePath := "commit/" + c + "/"
		toDownload, err := n.getIngestableFilesFromGCS(basePath)
		if err != nil {
			sklog.Warning(fmt.Errorf("Problem ingesting for commit %s: %s", c, err))
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
			parts := strings.Split(name, ".")
			if len(parts) >= 1 {
				ingestedJobs[parts[0]] = JobSummary{Name: parts[0], TotalLines: 100, MissedLines: 50}
			} else {
				sklog.Warningf("Unknown file to ingest: %s", name)
				continue
			}
			outpath := path.Join(n.dir, c, name)
			if fileExists(outpath) {
				// Destination file exists, no need to redownload.
				continue
			}

			if err := n.ingestFile(basePath, name, c); err != nil {
				sklog.Warningf("Problem ingesting file: %s", err)
			}
		}
		jobs := JobSummarySlice{}
		for _, j := range ingestedJobs {
			jobs = append(jobs, j)
		}
		// Sort jobs alphabetically for determinism
		sort.Sort(jobs)
		newResults = append(newResults, IngestedResults{Commit: c, Jobs: jobs})
	}

	n.resultsMutex.Lock()
	n.results = newResults
	n.resultsMutex.Unlock()
}

func (n *ingester) getIngestableFilesFromGCS(basePath string) ([]string, error) {
	toDownload := []string{}
	if err := n.gcsClient.AllFilesInDirectory(context.Background(), basePath, func(item *storage.ObjectAttrs) {
		toDownload = append(toDownload, item.Name)
	}); err != nil {
		return nil, fmt.Errorf("Could not get ingestable files from path %s: %s", basePath, err)
	}
	return toDownload, nil
}

func (n *ingester) ingestFile(basePath, name, commit string) error {
	dl := basePath + name
	if contents, err := n.gcsClient.GetFileContents(context.Background(), dl); err != nil {
		return fmt.Errorf("Could not download file %s, from GCS : %s", dl, err)

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
	}
	return nil
}

func (n *ingester) GetResults() []IngestedResults {
	return n.results
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else if err != nil {
		sklog.Warningf("Error getting file info about %s: %s", path, err)
		return true
	} else {
		return true
	}
}

func (s JobSummarySlice) Len() int           { return len(s) }
func (s JobSummarySlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s JobSummarySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
