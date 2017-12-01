package coverageingest

// The coverageingest package contains the code needed to download and interpret
// the results from our LLVM-based coverage tasks

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/coverage/go/common"
	"go.skia.org/infra/coverage/go/db"
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
	IngestCommits(context.Context, []*vcsinfo.LongCommit)

	// GetResults returns everything that was ingested on the last IngestCommits() call.
	GetResults() []IngestedResults
}

// The IngestedResults links information about a commit with the coverage information
// produced by a list of jobs.
type IngestedResults struct {
	Commit        *vcsinfo.ShortCommit     `json:"info"`
	Jobs          []common.CoverageSummary `json:"jobs"`
	TotalCoverage common.CoverageSummary   `json:"combined"`
}

// The gcsingester implements the Ingester interface with Google Cloud Storage (GCS)
type gcsingester struct {
	dir          string
	gcsClient    gcs.GCSClient
	results      []IngestedResults
	cache        db.CoverageCache
	resultsMutex sync.Mutex
}

// New returns an Ingester that is ready to be used.
func New(ingestionDir string, gcsClient gcs.GCSClient, cache db.CoverageCache) *gcsingester {
	return &gcsingester{
		gcsClient: gcsClient,
		dir:       ingestionDir,
		cache:     cache,
	}
}

// The function unTar will untar and unzip a .tar.gz file to a given output path.
// This tar file is assumed to be produced by our Coverage bots, which have
// a certain format (see defaultUnTar for details).
// It is a variable for easier mocking.
var unTar = defaultUnTar

func defaultUnTar(ctx context.Context, tarpath, outpath string) error {
	if _, err := fileutil.EnsureDirExists(outpath); err != nil {
		return fmt.Errorf("Could not set up directory to tar to: %s", err)
	}
	return exec.Run(ctx, &exec.Command{
		Name: "tar",
		// Strip components 6 removes /mnt/pd0/s/w/ir/coverage_html/ from the
		// tar file's internal folders.
		Args: []string{"xf", tarpath, "--strip-components=6", "-C", outpath},
	})
}

// The renderInfo struct contains information needed to create the combined reports.
type renderInfo struct {
	outputPath string
	commit     string
	jobName    string
}

// getCoverage returns the CoverageSummary from cache or calculates it and
// puts it into the cache. If there was any error, it is returned.
func (n *gcsingester) getCoverage(cacheKey string, ri renderInfo, folders ...string) (common.CoverageSummary, error) {
	if obj, ok := n.cache.CheckCache(cacheKey); ok {
		return obj, nil
	}
	if cov, err := calculateCoverage(ri, folders...); err != nil {
		return common.CoverageSummary{}, err
	} else {
		return cov, n.cache.StoreToCache(cacheKey, cov)
	}
}

// calcuateCoverage analyzes one or more folders of coverage data and combines them together
// to get a complete picture of the coverage. It is a variable for easier mocking.
// If the renderInfo's outputPath is not "", a coverage report will be generated there
// in addition to returning the CoverageSummary.
var calculateCoverage = defaultCalculateTotalCoverage

func defaultCalculateTotalCoverage(ri renderInfo, folders ...string) (common.CoverageSummary, error) {
	if len(folders) == 0 {
		return common.CoverageSummary{}, nil
	}
	if ri.outputPath != "" {
		if _, err := fileutil.EnsureDirExists(path.Join(ri.outputPath, "coverage")); err != nil {
			return common.CoverageSummary{}, fmt.Errorf("Could not create output directories: %s", err)
		}
	}
	totalLines := 0
	missedLines := 0

	// relPaths is a set of paths relative to the passed in folders of where
	// the coverage data is.
	relPaths := util.StringSet{}

	// Make a list of all files in all folders.  This is needed to make sure we analyze
	// all the files that may be run.  For example, the vulkan bots use vulkan specific
	// files that do not show up in the CPU only run.  So we must do this first pass
	// to make sure we collect all the files that we have data for.
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
			return common.CoverageSummary{}, fmt.Errorf("Error while walking directory %s: %s", f, err)
		}
	}

	// This will hold the information needed to create the summary page, that is, the coverage
	// data for each file.
	summaryData := coverageSummaryTemplateData{
		Commit:  ri.commit,
		JobName: ri.jobName,
	}

	// Go through all the relative files and figure out the coverage data for them.
	// We union together all the data for the same relative file (e.g. the CPU config's
	// coverage of DM.cpp and the GPU config's coverage of DM.cpp), then add that data
	// to our total summary.
	for rp, _ := range relPaths {
		linesCovered := &coverageData{}
		for _, f := range folders {
			p := path.Join(f, rp)
			contents, err := ioutil.ReadFile(p)
			if err != nil {
				// The file might not exist for all configurations (see the
				// above vulkan example), so we simply skip a file that we don't see.
				continue
			}
			newlyCovered := parseLinesCovered(string(contents))
			linesCovered = linesCovered.Union(newlyCovered)
		}

		normPath, shouldSummarize := normalizePath(rp)
		if !shouldSummarize {
			continue
		}
		totalLines += linesCovered.TotalExecutable()
		missedLines += linesCovered.MissedExecutable()

		// Write out an html file representing the combined coverage of the file represented
		// by the given relative path to ri.outputPath if ri.outputPath is defined.
		if ri.outputPath != "" {
			percent := "--"
			if tl, ml := linesCovered.TotalExecutable(), linesCovered.MissedExecutable(); tl != 0 {
				percent = fmt.Sprintf("%1.2f", 100.0*float32(tl-ml)/float32(tl))
			}

			summaryData.Files = append(summaryData.Files, fileSummaryTemplateData{
				FileName:     normPath,
				CoveredLines: linesCovered.TotalExecutable() - linesCovered.MissedExecutable(),
				TotalLines:   linesCovered.TotalExecutable(),
				PercentLines: percent,
			})

			dest := path.Join(ri.outputPath, "coverage", normPath+".html")
			if err := fileutil.EnsureDirPathExists(dest); err != nil {
				return common.CoverageSummary{}, err
			}
			content, err := linesCovered.ToHTMLPage(CoverageFileData{
				FileName: rp,
				Commit:   ri.commit,
				JobName:  ri.jobName,
			})
			if err != nil {
				return common.CoverageSummary{}, err
			}
			if err := ioutil.WriteFile(dest, []byte(content), 0644); err != nil {
				return common.CoverageSummary{}, err
			}
		}
	}

	// Write out an html file summarizing the coverage of all the files if ri.outputPath
	// is defined.
	if ri.outputPath != "" {
		// Sort for determinism and ease of reading.
		sort.Sort(summaryData.Files)
		b := bytes.Buffer{}
		if err := HTML_TEMPLATE_SUMMARY.Execute(&b, summaryData); err != nil {
			return common.CoverageSummary{}, err
		}
		if err := ioutil.WriteFile(path.Join(ri.outputPath, "index.html"), []byte(b.String()), 0644); err != nil {
			return common.CoverageSummary{}, err
		}
	}

	return common.CoverageSummary{TotalLines: totalLines, MissedLines: missedLines}, nil
}

// normalizePath returns the path with any unnecessary prefix stripped off.
// For example, LLVM outputs the absolute path to all these files, which includes
// the path to the source folder on the bots - we strip this off. normalizePath
// also returns true if this file should be included in our analysis (e.g. skip
// third_party).
func normalizePath(p string) (string, bool) {
	p = strings.TrimPrefix(p, "/mnt/pd0/work/skia/")
	// This removes things like /usr/lib/fontconfig, some created things and third_party.
	// TODO(kjlubick): Keep third_party in and make it configurable from the UI what to show.
	return p, !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "out") && !strings.HasPrefix(p, "third_party")
}

// IngestCommits fulfills the Ingester interface.
func (n *gcsingester) IngestCommits(ctx context.Context, commits []*vcsinfo.LongCommit) {
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
				if err := n.ingestFile(ctx, basePath, name, c.Hash); err != nil {
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
		// We go through the list of all the jobs we know of and analyze their coverage
		// individually and then add them to the list to be joined together in a combined
		// fashion.
		jobs := common.CoverageSummarySlice{}
		toCombine := []string{}
		for job, folder := range toSummarize {
			cov, err := n.getCoverage(makeCacheKey(c.Hash, job), renderInfo{}, folder)
			if err != nil {
				sklog.Warningf("Was unable to create a coverage data: %s", err)
				continue
			}
			cov.Name = job
			jobs = append(jobs, cov)
			toCombine = append(toCombine, folder)
		}
		// Sort jobs alphabetically for determinism
		sort.Sort(jobs)
		sort.Strings(toCombine)

		// Mimic the structure that LLVM outputs, e.g.
		// .../[hash]/[name]/html/
		//                        index.html
		//                        coverage/
		//                                 foo.cpp.html
		//                                 bar.cpp.html
		ri := renderInfo{
			outputPath: path.Join(n.dir, c.Hash, "Combined", "html"),
			commit:     c.Hash,
			jobName:    "Combined",
		}

		totalCoverage, err := n.getCoverage(makeCacheKey(c.Hash, toCombine...), ri, toCombine...)
		if err != nil {
			sklog.Errorf("Was unable to create a combined summary: %s", err)
		}
		newResults = append(newResults, IngestedResults{Commit: c.ShortCommit, Jobs: jobs, TotalCoverage: totalCoverage})
		sklog.Infof("Ingestion completed for commit %s - %s", c.ShortCommit.Hash, c.ShortCommit.Author)
	}
	n.resultsMutex.Lock()
	defer n.resultsMutex.Unlock()
	n.results = newResults
}

// makeCacheKey returns a unique key for one or more job names and a given commit.
// It is somewhat human readable.
func makeCacheKey(commit string, names ...string) string {
	// for readability, if theres' one name, use it, otherwise, combine the names of the
	// folders being analyzed and hash them together.  This "invalidates" the cache if 2
	// jobs finish and report coverage, then a 3rd finishes and is ready to be analyzed.
	if len(names) == 1 {
		return names[0] + ":" + commit
	}
	toHash := strings.Join(names, "|")
	return fmt.Sprintf("Combined(%x):%s", md5.Sum([]byte(toHash)), commit)
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
func (n *gcsingester) ingestFile(ctx context.Context, basePath, name, commit string) error {
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
			if err := unTar(ctx, outpath, path.Join(n.dir, commit, parts[0], parts[1])); err != nil {
				return fmt.Errorf("Could not untar %s: %s", outpath, err)
			}
		}
		return nil
	}
}

// GetResults fulfills the Ingester interface
func (n *gcsingester) GetResults() []IngestedResults {
	n.resultsMutex.Lock()
	defer n.resultsMutex.Unlock()
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
