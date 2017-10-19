package ingester

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

type ingester struct {
	dir       string
	gcsClient gcs.GCSClient
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

func (n *ingester) ingestCommits(commits []string) {
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
		for _, name := range toDownload {
			outpath := path.Join(n.dir, c, name)
			if fileExists(outpath) {
				// Destination file exists, no need to redownload.
				continue
			}

			dl := basePath + name
			if contents, err := n.gcsClient.GetFileContents(context.Background(), dl); err != nil {
				sklog.Warningf("Could not download file %s, from GCS : %s", dl, err)
				continue
			} else {
				outpath := path.Join(n.dir, c, name)
				file, err := os.Create(outpath)
				if err != nil {
					sklog.Warningf("Could not open file %s for writing", outpath)
					continue
				}
				defer util.Close(file)
				if i, err := file.Write(contents); err != nil {
					sklog.Warningf("Could not write completely to %s. Only wrote %d bytes: %s", outpath, i, err)
				}
				if strings.HasSuffix(name, "tar") {
					// Split My-Config-Name.type.tar into 3 parts.  type is "text" or "html"
					parts := strings.Split(name, ".")
					if len(parts) != 3 {
						sklog.Warningf("Invalid tar name to ingest %s - must have 3 parts", name)
					}
					if err := unTar(outpath, path.Join(n.dir, c, parts[0], parts[1])); err != nil {
						sklog.Warningf("Could not untar %s: %s", outpath, err)
					}
				}
			}
		}
	}
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
