package builder

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/config"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	GOOD_BUILDS_FILENAME = "goodbuilds.txt"

	// PRESERVE_DURATION is used to determine if an LKGR commit should be
	// preserved.  i.e. if a the distance between two commits is greater than
	// PRESERVER_DURATION then they both should be preserved.
	PRESERVE_DURATION = 30 * 24 * time.Hour

	// PRESERVE_COUNT is the max number of fresh LKGR builds to keep around.
	// Always keep odd so the first and last hashes are always preserved.
	PRESERVE_COUNT = 33

	// DECIMATION_PERIOD is the time between decimation runs.
	DECIMATION_PERIOD = time.Hour
)

// errors
var (
	AlreadyExistsErr = errors.New("Checkout already exists.")
)

var (
	branchRegex = regexp.MustCompile("^refs/heads/chrome/m([0-9]+)$")
)

// Builder is for building versions of the Skia library and then compiling and
// running fiddles against those built versions.
//
//    fiddleRoot - The root directory where fiddle stores its files. See DESIGN.md.
//    depotTools - The directory where depot_tools is checked out.
type Builder struct {
	fiddleRoot string
	depotTools string
	repo       vcsinfo.VCS

	// hashes is a cache of the hashes returned from Available.
	hashes []string

	// current is the current commit we are building at.
	current *vcsinfo.LongCommit

	// mutex protects access to hashes, current, and GOOD_BUILDS_FILENAME.
	mutex sync.Mutex
}

// New returns a new Builder instance.
//
//    fiddleRoot - The root directory where fiddle stores its files. See DESIGN.md.
//    depotTools - The directory where depot_tools is checked out.
//    repo - A vcs to pull hash info from.
func New(fiddleRoot, depotTools string, repo vcsinfo.VCS) *Builder {
	b := &Builder{
		fiddleRoot: fiddleRoot,
		depotTools: depotTools,
		repo:       repo,
	}
	go b.startDecimation()
	b.updateCurrent()

	return b
}

// branch is used to sort the chrome branches in the Skia repo.
type branch struct {
	N    int
	Name string
	Hash string
}

// branchSlice is a utility class for sorting slices of branch.
type branchSlice []branch

func (p branchSlice) Len() int           { return len(p) }
func (p branchSlice) Less(i, j int) bool { return p[i].N > p[j].N }
func (p branchSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// prepDirectory adds the 'versions' directory to the fiddleRoot
// and returns the full path of that directory.
func prepDirectory(fiddleRoot string) (string, error) {
	versions := path.Join(fiddleRoot, "versions")
	if err := os.MkdirAll(versions, 0777); err != nil {
		return "", fmt.Errorf("Failed to create FIDDLE_ROOT/versions dir: %s", err)
	}
	return versions, nil
}

// buildLib, given a directory that Skia is checked out into, builds libskia.a
// and fiddle_main.o.
func buildLib(checkout, depotTools string) error {
	glog.Info("Starting CMakeBuild")
	if err := buildskia.CMakeBuild(checkout, depotTools, config.BUILD_TYPE); err != nil {
		return fmt.Errorf("Failed cmake build: %s", err)
	}

	glog.Info("Building fiddle_main.o")
	files := []string{
		filepath.Join(checkout, "tools", "fiddle", "fiddle_main.cpp"),
	}
	if err := buildskia.CMakeCompile(checkout, path.Join(checkout, "cmakeout", "fiddle_main.o"), files, []string{}, config.BUILD_TYPE); err != nil {
		return fmt.Errorf("Failed cmake build of fiddle_main: %s", err)
	}
	return nil
}

// BuildLatestSkia builds the LKGR of Skia in the given fiddleRoot directory.
//
// The library will be checked out into fiddleRoot + "/" + githash, where githash
// is the githash of the LKGR of Skia.
//
//    force - If true then checkout and build even if the directory already exists.
//    head - If true then build Skia at HEAD, otherwise build Skia at LKGR.
//    deps - If true then install Skia dependencies.
//
// Returns the commit info for the revision of Skia checked out.
// Returns an error if any step fails, or return AlreadyExistsErr if
// the target checkout directory already exists and force is false.
func (b *Builder) BuildLatestSkia(force bool, head bool, deps bool) (*vcsinfo.LongCommit, error) {
	versions, err := prepDirectory(b.fiddleRoot)
	if err != nil {
		return nil, err
	}

	githash := ""
	if head {
		if githash, err = buildskia.GetSkiaHead(nil); err != nil {
			return nil, fmt.Errorf("Failed to retrieve Skia HEAD: %s", err)
		}
	} else {
		if githash, err = buildskia.GetSkiaHash(nil); err != nil {
			return nil, fmt.Errorf("Failed to retrieve Skia LKGR: %s", err)
		}
	}
	checkout := path.Join(versions, githash)

	fi, err := os.Stat(checkout)
	// If the file is present and a directory then only proceed if 'force' is true.
	if err == nil && fi.IsDir() == true && !force {
		return nil, AlreadyExistsErr
	}

	ret, err := buildskia.DownloadSkia("", githash, checkout, b.depotTools, false, deps)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch: %s", err)
	}

	if err := buildLib(checkout, b.depotTools); err != nil {
		return nil, err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.hashes = append(b.hashes, githash)
	b.updateCurrent()
	fb, err := os.OpenFile(filepath.Join(b.fiddleRoot, GOOD_BUILDS_FILENAME), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s for writing: %s", GOOD_BUILDS_FILENAME, err)
	}
	defer util.Close(fb)
	_, err = fmt.Fprintf(fb, "%s\n", githash)
	if err != nil {
		return nil, fmt.Errorf("Failed to write %s: %s", GOOD_BUILDS_FILENAME, err)
	}
	return ret, nil
}

// updateCurrent updates the value of b.current with the new gitinfo for the most recent build.
//
// Or a mildly informative stand-in if somehow the update fails.
func (b *Builder) updateCurrent() {
	allBuilds, err := b.AvailableBuilds()
	b.mutex.Lock()
	defer b.mutex.Unlock()
	fallback := &vcsinfo.LongCommit{ShortCommit: &vcsinfo.ShortCommit{Hash: "unknown"}}
	if err != nil {
		glog.Errorf("Failed to get list of available builds: %s", err)
		if b.current == nil {
			b.current = fallback
		}
		return
	}
	details, err := b.repo.Details(allBuilds[0], true)
	if err != nil {
		glog.Errorf("Unable to retrieve build info: %s", err)
		if b.current == nil {
			b.current = fallback
		}
		return
	}
	b.current = details
}

// AvailableBuilds returns a list of git hashes, all the versions of Skia that
// can be built against. This returns the list with the newest builds first.
// The list will always be of length > 1, otherwise and error is returned.
func (b *Builder) AvailableBuilds() ([]string, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if len(b.hashes) > 0 {
		return b.hashes, nil
	}
	fi, err := os.Open(filepath.Join(b.fiddleRoot, GOOD_BUILDS_FILENAME))
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s for reading: %s", GOOD_BUILDS_FILENAME, err)
	}
	defer util.Close(fi)
	buf, err := ioutil.ReadAll(fi)
	if err != nil {
		return nil, fmt.Errorf("Failed to read: %s", err)
	}
	hashes := strings.Split(string(buf), "\n")
	revHashes := []string{}
	for i := len(hashes) - 1; i >= 0; i-- {
		h := hashes[i]
		if h != "" {
			revHashes = append(revHashes, h)
		}
	}
	b.hashes = revHashes
	if len(b.hashes) == 0 {
		return nil, fmt.Errorf("List of available builds is empty.")
	}
	return revHashes, nil
}

func (b *Builder) Current() *vcsinfo.LongCommit {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.current
}

// BuildLatestSkiaChromeBranch builds the most recent branch of Skia for Chrome
// in the given fiddleRoot directory.
//
// The library will be checked out into fiddleRoot + "/" + mNN, where mNN
// is the short name of the branch for Chrome. The mNN is chosen as the largest
// NN from all the branches named refs/heads/chrome/m[0-9]+.
//
//   force - If true then checkout and build even if the directory already exists.
//
// Returns the commit info for the revision of Skia checked out.
// Returns an error if any step fails, or return AlreadyExistsErr if
// the target checkout directory already exists and force is false.
func (b *Builder) BuildLatestSkiaChromeBranch(force bool) (string, *vcsinfo.LongCommit, error) {
	versions, err := prepDirectory(b.fiddleRoot)
	if err != nil {
		return "", nil, err
	}
	branches, err := buildskia.GetSkiaBranches(nil)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to retrieve branch info: %s", err)
	}
	if len(branches) == 0 {
		return "", nil, fmt.Errorf("There must be at least one branch.")
	}

	branchNums := []branch{}
	for name, br := range branches {
		if match := branchRegex.FindStringSubmatch(name); match != nil {
			n, err := strconv.Atoi(match[1])
			if err != nil {
				glog.Errorf("Failed to parse branch number: %s", err)
				continue
			}
			branchNums = append(branchNums, branch{N: n, Name: name, Hash: br.Value})
		}
	}
	sort.Sort(branchSlice(branchNums))
	if len(branchNums) == 0 {
		return "", nil, fmt.Errorf("Failed to find any appropriate branches.")
	}

	branchName := fmt.Sprintf("m%d", branchNums[0].N)
	glog.Infof("Target branch number is: %d", branchName)

	checkout := path.Join(versions, branchName)

	fi, err := os.Stat(checkout)
	// If the file is present and a directory then only proceed if 'force' is true.
	if err == nil && fi.IsDir() == true && !force {
		return "", nil, AlreadyExistsErr
	}

	res, err := buildskia.DownloadSkia(branchNums[0].Name, branchNums[0].Hash, checkout, b.depotTools, false, false)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to fetch: %s", err)
	}

	if err := buildLib(checkout, b.depotTools); err != nil {
		return "", nil, err
	}
	return branchName, res, nil
}

func (b *Builder) writeNewGoodBuilds(hashes []string) error {
	if len(hashes) < 1 {
		return fmt.Errorf("At least one good build must be kept around.")
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()

	revHashes := []string{}
	for i := len(hashes) - 1; i >= 0; i-- {
		h := hashes[i]
		if h != "" {
			revHashes = append(revHashes, h)
		}
	}
	b.hashes = hashes
	fb, err := os.Create(filepath.Join(b.fiddleRoot, GOOD_BUILDS_FILENAME))
	if err != nil {
		return fmt.Errorf("Failed to open %s for writing: %s", GOOD_BUILDS_FILENAME, err)
	}
	defer util.Close(fb)
	if _, err := fb.Write([]byte(strings.Join(revHashes, "\n") + "\n")); err != nil {
		return fmt.Errorf("Failed to write %s: %s", GOOD_BUILDS_FILENAME, err)
	}
	return nil
}

func (b *Builder) startDecimation() {
	decimateLiveness := metrics2.NewLiveness("decimate")
	decimateFailures := metrics2.GetCounter("decimate-failed", nil)
	for _ = range time.Tick(DECIMATION_PERIOD) {
		hashes, err := b.AvailableBuilds()
		if err != nil {
			glog.Errorf("Failed to get available builds while decimating: %s", err)
			decimateFailures.Inc(1)
			continue
		}
		keep, remove, err := decimate(hashes, b.repo, PRESERVE_COUNT)
		if err != nil {
			glog.Errorf("Failed to calc removals while decimating: %s", err)
			decimateFailures.Inc(1)
			continue
		}
		for _, hash := range remove {
			glog.Infof("Decimate: Beginning %s", hash)
			if err := os.RemoveAll(filepath.Join(b.fiddleRoot, "versions", hash)); err != nil {
				glog.Errorf("Failed to remove directory for %s: %s", hash, err)
				continue
			}
			glog.Infof("Decimate: Finished %s", hash)
		}
		if err := b.writeNewGoodBuilds(keep); err != nil {
			continue
		}
		decimateFailures.Reset()
		decimateLiveness.Reset()
	}
}

// decimate returns a list of hashes to keep, the list to remove,
// and an error if one occurred.
//
// The algorithm is:
//   Preserve all hashes that are spaced one month apart.
//   Then if there are more than 'limit' remaining hashes
//   remove every other one to bring the count down to 'limit'/2.
//
func decimate(hashes []string, vcs vcsinfo.VCS, limit int) ([]string, []string, error) {
	keep := []string{}
	remove := []string{}

	// The hashes are in reverse chronological order, so we start at the end
	// and work back until we start to find hashes that are less than
	// PRESERVE_DURATION apart. Once we find that spot set oldiesBegin
	// to that index.
	oldiesBegin := len(hashes)
	c, err := vcs.Details(hashes[len(hashes)-1], true)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get hash details: %s", err)
	}
	lastTS := c.Timestamp
	for i := len(hashes) - 2; i > 0; i-- {
		c, err := vcs.Details(hashes[i], true)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to get hash details: %s", err)
		}
		if c.Timestamp.Sub(lastTS) < PRESERVE_DURATION {
			glog.Infof("Decimation: Time %v between %q and %q", c.Timestamp.Sub(lastTS), hashes[i], hashes[i+1])
			break
		}
		lastTS = c.Timestamp
		oldiesBegin = i
	}

	// Now that we know where the old hashes that we want to preserve are, we
	// will chop them off and ignore them for the rest of the decimation process.
	oldies := hashes[oldiesBegin:]
	hashes = hashes[:oldiesBegin]

	// Only do decimation if we have enough fresh hashes.
	if len(hashes) < limit {
		return append(hashes, oldies...), remove, nil
	}
	for i, h := range hashes {
		if i%2 == 0 {
			keep = append(keep, h)
		} else {
			remove = append(remove, h)
		}
	}
	// Once done with decimation add the oldies back into the list of hashes to keep.
	return append(keep, oldies...), remove, nil
}
