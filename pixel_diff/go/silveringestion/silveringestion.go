package silveringestion

import (
	"encoding/json"
	"net/http"
	"path"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/pixel_diff/go/config"
	"go.skia.org/infra/pixel_diff/go/dmstore"
)

const (
	// Configuration option that identifies the directory of the boltDB instance.
	CONFIG_BOLTDIR = "BoltDir"

	// Configuration option that identifies the name of the boltDB instance.
	CONFIG_BOLTNAME = "BoltName"


)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_SILVER, newSilverProcessor)
}

// silverProcessor implements the ingestion.Processor interface for silver.
type silverProcessor struct {
	vcs          vcsinfo.VCS
	screenshots  *bolt.DB
	dmstore      *dmstore.DMStore
}

// newSilverProcessor implements the ingestion.Constructor signature.
// dmstore.Init() needs to be called before starting ingestion so that
// dmstore.Default can be intialized properly.
func newSilverProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	boltdir := config.ExtraParams[CONFIG_BOLTDIR]
	boltname := config.ExtraParams[CONFIG_BOLTNAME]

	boltdir, err := fileutil.EnsureDirExists(boltdir)
	if err != nil {
		return nil, err
	}

	screenshots, err := bolt.Open(path.Join(boltdir, boltname), 0600, nil)
	if err != nil {
		return nil, err
	}

	return &silverProcessor {
		vcs:           vcs,
		screenshots:   screenshots,
		dmstore:       dmstore.Default,
	}, nil
}

// See the ingestion.Processor interface.
func (s *silverProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}

	// Parse the JSON file.
	ctResults, err := ParseCTResultsFromReader(r, resultsFile.Name())
	if err != nil {
		return err
	}

	err = s.screenshots.Update(func(tx *bolt.Tx) error {
		// Create bucket using the RunID.
		b, err := tx.CreateBucketIfNotExists([]byte(ctResults.RunID))
		if err != nil {
			return err
		}

		// Records in bucket: key = URl,
		// value = [nopatch image processed, withpatch image processed]
		for _, screenshot := range ctResults.Screenshots {
			bytes := b.Get([]byte(screenshot.Filename))
			processed := make([]bool, 2)
			if bytes != nil {
				if err := json.Unmarshal(bytes, &processed); err != nil {
					return err
				}
			}

			if screenshot.Type == "nopatch" {
				processed[0] = true
			} else if screenshot.Type == "withpatch" {
				processed[1] = true
			}

			// If both nopatch and withpatch images for a URL have been processed,
			// add it to the dmstore so the diff can be calculated.
			if processed[0] && processed[1] {
				if err := s.dmstore.Add(ctResults.RunID, screenshot.Filename); err != nil {
					return err
				}
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
	})
	return err
}

// See the ingestion.Processor interface.
func (s *silverProcessor) BatchFinished() error { return nil }
