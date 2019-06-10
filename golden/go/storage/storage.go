package storage

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/tryjobs"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// maxNSparseCommits is the maximum number of commits we are considering when condensing a
	// sparse tile into a dense tile by removing commits that contain no data.
	// This should be changed or made a config option when we consider going back more commits makes
	// sense.
	maxNSparseCommits = 3000
)

// Storage is a container struct for the various storage objects we are using.
// It is intended to reduce parameter lists as we pass around storage objects.
type Storage struct {
	DiffStore         diff.DiffStore
	ExpectationsStore expstorage.ExpectationsStore
	IgnoreStore       ignore.IgnoreStore
	EventBus          eventbus.EventBus
	TryjobStore       tryjobstore.TryjobStore
	TryjobMonitor     tryjobs.TryjobMonitor
	GerritAPI         gerrit.GerritInterface
	GCSClient         GCSClient
	Baseliner         baseline.Baseliner
	VCS               vcsinfo.VCS
	WhiteListQuery    paramtools.ParamSet
	IsAuthoritative   bool
	SiteURL           string

	// IsSparseTile indicates that new tiles should be condensed by removing commits that have no data.
	IsSparseTile bool

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int

	// Internal variables used to cache tiles.
	lastCpxTile   types.ComplexTile
	lastTimeStamp time.Time
	mutex         sync.Mutex
}

// LoadWhiteList loads the given JSON5 file that defines that query to
// whitelist traces. If the given path is empty or the file cannot be parsed
// an error will be returned.
func (s *Storage) LoadWhiteList(fName string) error {
	if fName == "" {
		return fmt.Errorf("No white list file provided.")
	}

	f, err := os.Open(fName)
	if err != nil {
		return fmt.Errorf("Unable open file %s. Got error: %s", fName, err)
	}
	defer util.Close(f)

	if err := json5.NewDecoder(f).Decode(&s.WhiteListQuery); err != nil {
		return err
	}

	// Make sure the whitelist is not empty.
	empty := true
	for _, values := range s.WhiteListQuery {
		if empty = len(values) == 0; !empty {
			break
		}
	}
	if empty {
		return fmt.Errorf("Whitelist in %s cannot be empty.", fName)
	}
	sklog.Infof("Whitelist loaded from %s", fName)
	return nil
}
