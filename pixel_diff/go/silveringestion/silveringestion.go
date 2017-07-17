package silveringestion

import (
	"encoding/json"
	"fmt"
	"io"
	"path"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// Used to keep track of which screenshots for a particular URL have been
	// processed.
	NOPATCHIDX = iota
	WITHPATCHIDX
	NOPATCH = "nopatch"
	WITHPATCH = "withpatch"
)

// The JSON output from CT looks like this:
// {
//  “run_id” : “{userid-timestamp}”,
//  “patch” : “{link to patch}”,
//  “screenshots” : [
//    {
//      “type” : “{nopatch/withpatch}”,
//      “filename” : “{GS filename}”,
//    }, ...
//  ]
// }

// Screenshot contains the information for a screenshot taken by CT.
type Screenshot struct {
	// Type specifies whether the screenshot was taken without the patch or with
	// the patch.
	Type     string `json:"type"`

	// Filename is the name of the screenshot, as stored in GS.
	Filename string `json:"filename"`
}

// CTResults is the top level structure for decoding CT pixel diff JSON output.
type CTResults struct {
	// RunID specifies the unique ID for the CT run, in the form userid-timestamp.
	RunID       string         `json:"run_id"`

	// Patch is a link to the patch used to create the pixel diff run.
	Patch       string         `json:"patch"`

	// Screenshots lists the screenshots taken and accounted for in the JSON file.
	Screenshots []*Screenshot  `json:"screenshots"`

	// name is the name/path of the file where the data came from.
	name        string
}

// Parses CT pixel diff JSON output and returns a CTResults object.
func parseCTResultsFromReader(r io.ReadCloser, name string) (*CTResults, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	ctResults := &CTResults{}
	if err := dec.Decode(ctResults); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	ctResults.name = name
	return ctResults, nil
}

// silverProcessor implements the ingestion.Processor interface for silver.
type silverProcessor struct {
	screenshots *bolt.DB
	diffStore   diff.DiffStore

	// TODO(lchoi): Change when dmStore package is implemented.
	dmStore     interface{}
}

// newSilverProcessor takes in a DiffStore instance and strings used to specify
// the boltDB screenshots directory and instance to return the ingestion.Processor
// for silver's ingestion process
func newSilverProcessor(diffStore diff.DiffStore, boltdir, boltname string) (ingestion.Processor, error) {
	// Make sure directory for screenshots boltDB instance exists.
	boltdir, err := fileutil.EnsureDirExists(boltdir)
	if err != nil {
		return nil, err
	}

	// Create the screenshots boltDB instance.
	screenshots, err := bolt.Open(path.Join(boltdir, boltname), 0600, nil)
	if err != nil {
		return nil, err
	}

	return &silverProcessor {
		screenshots: screenshots,
		diffStore:   diffStore,

		// TODO(lchoi): Change when dmStore package is implemented.
		dmStore:     nil,
	}, nil
}

// See the ingestion.Processor interface.
func (s *silverProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}

	// Parse the JSON file.
	ctResults, err := parseCTResultsFromReader(r, resultsFile.Name())
	if err != nil {
		return err
	}

	updateFn := func(tx *bolt.Tx) error {
		// Create bucket using the RunID.
		b, err := tx.CreateBucketIfNotExists([]byte(ctResults.RunID))
		if err != nil {
			return err
		}

		// Records in bucket: key = URL,
		// value = [nopatch image processed, withpatch image processed]
		for _, screenshot := range ctResults.Screenshots {
			bytes := b.Get([]byte(screenshot.Filename))
			processed := make([]bool, 2)
			if bytes != nil {
				if err := json.Unmarshal(bytes, &processed); err != nil {
					return err
				}
			}

			if screenshot.Type == NOPATCH {
				processed[NOPATCHIDX] = true
			} else if screenshot.Type == WITHPATCH {
				processed[WITHPATCHIDX] = true
			}

			// If both nopatch and withpatch images for a URL have been processed,
			// use the diffstore to calculate diff metrics and add them to the DMStore.
			if processed[NOPATCHIDX] && processed[WITHPATCHIDX] {
				// TODO(lchoi): Uncomment when dmStore package is implemented.
				// noPatchImg, withPatchImg := getNoAndWithPatch(ctResults.RunID, screenshot.Filename)
				// diff, err := s.diffStore.Get(diff.PRIORITY_NOW, noPatchImg, []string{withPatchImg})
				// if err != nil {
				// 	return err
				// }
				// diffMetrics := diff[withPatchImg]
				// if err = s.dmStore.Add(diffMetrics); err != nil {
				// 	return err
				// }
			}

			encoded, err := json.Marshal(processed)
			if err != nil {
				return err
			}

			if err := b.Put([]byte(screenshot.Filename), encoded); err != nil {
				return err
			}
		}
		return nil
	}

	return s.screenshots.Update(updateFn)
}

// See the ingestion.Processor interface.
func (s *silverProcessor) BatchFinished() error { return nil }

// Returns GS paths of nopatch and withpatch images.
func getNoAndWithPatch(runID, filename string) (string, string) {
	return runID + "/nopatch/"+ filename, runID + "/withpatch/" + filename
}

