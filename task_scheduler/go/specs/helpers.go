package specs

/*
	Helper functions for client repos.
*/

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags.
	test = flag.Bool("test", false, "Run in test mode: verify that the output hasn't changed.")
)

// GetCheckoutRoot returns the path of the root of the checkout.
func GetCheckoutRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(cwd); err != nil {
			return "", err
		}
		// TODO(borenet): Should we verify that this is the
		// correct checkout and not something else?

		// Check for infra/bots dir.
		s, err := os.Stat(filepath.Join(cwd, "infra", "bots"))
		if err == nil && s.IsDir() {
			return cwd, nil
		}
		// Check for .git dir.
		s, err = os.Stat(filepath.Join(cwd, ".git"))
		if err == nil && s.IsDir() {
			return cwd, nil
		}

		// Move up a level.
		cwd = filepath.Clean(filepath.Join(cwd, ".."))

		// Stop if we're at the filesystem root.
		// Per filepath.Clean docs, cwd will end in a slash only if it
		// represents a root directory.
		if strings.HasSuffix(cwd, string(filepath.Separator)) {
			return "", fmt.Errorf("Unable to find repository root.")
		}
	}
}

// TasksCfgBuilder is a helper struct used for building a TasksCfg.
type TasksCfgBuilder struct {
	assetsDir    string
	cfg          *TasksCfg
	cipdPackages map[string]*CipdPackage
	root         string
}

// NewTasksCfgBuilder returns a TasksCfgBuilder instance.
func NewTasksCfgBuilder() (*TasksCfgBuilder, error) {
	common.Init()

	// Create the config.
	cfg := &TasksCfg{
		Jobs:  map[string]*JobSpec{},
		Tasks: map[string]*TaskSpec{},
	}

	root, err := GetCheckoutRoot()
	if err != nil {
		return nil, err
	}

	return &TasksCfgBuilder{
		cfg:          cfg,
		cipdPackages: map[string]*CipdPackage{},
		root:         root,
	}, nil
}

// MustNewTasksCfgBuilder returns a TasksCfgBuilder instance. Panics on error.
func MustNewTasksCfgBuilder() *TasksCfgBuilder {
	b, err := NewTasksCfgBuilder()
	if err != nil {
		sklog.Fatal(err)
	}
	return b
}

// CheckoutRoot returns the path to the root of the client checkout.
func (b *TasksCfgBuilder) CheckoutRoot() string {
	return b.root
}

// SetAssetsDir sets the directory path used for assets.
func (b *TasksCfgBuilder) SetAssetsDir(assetsDir string) {
	b.assetsDir = assetsDir
}

// AddTask adds a TaskSpec to the TasksCfgBuilder. Returns an error if the
// config already contains a Task with the same name and a different
// implementation.
func (b *TasksCfgBuilder) AddTask(name string, t *TaskSpec) error {
	// Return an error if the task contains duplicate dimensions, which will
	// cause it to be rejected by Swarming.
	dims := make(map[string]bool, len(t.Dimensions))
	for _, dim := range t.Dimensions {
		if _, ok := dims[dim]; ok {
			return fmt.Errorf("Dimension %q is duplicated for task %s", dim, name)
		}
		dims[dim] = true
	}
	// Ensure that we don't already have a different definition for this task
	// name.
	if old, ok := b.cfg.Tasks[name]; ok {
		if !reflect.DeepEqual(old, t) {
			return fmt.Errorf("Config already contains a Task named %q with a different implementation!\nHave:\n%v\n\nGot:\n%v", name, old, t)
		}
		return nil
	}
	b.cfg.Tasks[name] = t
	return nil
}

// MustAddTask adds a TaskSpec to the TasksCfgBuilder and panics on failure.
func (b *TasksCfgBuilder) MustAddTask(name string, t *TaskSpec) {
	if err := b.AddTask(name, t); err != nil {
		sklog.Fatal(err)
	}
}

// AddJob adds a JobSpec to the TasksCfgBuilder.
func (b *TasksCfgBuilder) AddJob(name string, j *JobSpec) error {
	if _, ok := b.cfg.Jobs[name]; ok {
		return fmt.Errorf("Config already contains a Job named %q", name)
	}
	b.cfg.Jobs[name] = j
	return nil
}

// MustAddJob adds a JobSpec to the TasksCfgBuilder and panics on failure.
func (b *TasksCfgBuilder) MustAddJob(name string, j *JobSpec) {
	if err := b.AddJob(name, j); err != nil {
		sklog.Fatal(err)
	}
}

// GetCipdPackageFromAsset reads the version information for the given asset
// and returns a CipdPackage instance.
func (b *TasksCfgBuilder) GetCipdPackageFromAsset(assetName string) (*CipdPackage, error) {
	if pkg, ok := b.cipdPackages[assetName]; ok {
		return pkg, nil
	}
	assetsDir := b.assetsDir
	if assetsDir == "" {
		assetsDir = filepath.Join(b.root, "infra", "bots", "assets")
	}
	versionFile := filepath.Join(assetsDir, assetName, "VERSION")
	contents, err := ioutil.ReadFile(versionFile)
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(string(contents))
	pkg := &CipdPackage{
		Name:    fmt.Sprintf("skia/bots/%s", assetName),
		Path:    assetName,
		Version: fmt.Sprintf("version:%s", version),
	}
	b.cipdPackages[assetName] = pkg
	return pkg, nil
}

// MustGetCipdPackageFromAsset reads the version information for the given asset
// and returns a CipdPackage instance. Panics on failure.
func (b *TasksCfgBuilder) MustGetCipdPackageFromAsset(assetName string) *CipdPackage {
	pkg, err := b.GetCipdPackageFromAsset(assetName)
	if err != nil {
		sklog.Fatal(err)
	}
	return pkg
}

// Finish validates and writes out the TasksCfg, or, if the --test flag is
// provided, verifies that the contents have not changed.
func (b *TasksCfgBuilder) Finish() error {
	// Sort the elements whose order is not important to maintain
	// consistency.
	for _, t := range b.cfg.Tasks {
		sort.Slice(t.Caches, func(i, j int) bool {
			return t.Caches[i].Name < t.Caches[j].Name
		})
		sort.Slice(t.CipdPackages, func(i, j int) bool {
			return t.CipdPackages[i].Name < t.CipdPackages[j].Name
		})
		sort.Strings(t.Dependencies)
		sort.Strings(t.Outputs)
	}
	for _, j := range b.cfg.Jobs {
		sort.Strings(j.TaskSpecs)
	}

	// Validate the config.
	if err := b.cfg.Validate(); err != nil {
		return err
	}

	enc, err := EncodeTasksCfg(b.cfg)
	if err != nil {
		return err
	}

	// Write the tasks.json file.
	outFile := filepath.Join(b.root, TASKS_CFG_FILE)
	if *test {
		// Don't write the file; read it and compare.
		expect, err := ioutil.ReadFile(outFile)
		if err != nil {
			return err
		}
		if !bytes.Equal(expect, enc) {
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(expect)),
				B:        difflib.SplitLines(string(enc)),
				FromFile: "Expected",
				ToFile:   "Actual",
				Context:  3,
				Eol:      "\n",
			})
			if err != nil {
				diff = fmt.Sprintf("<failed to obtain diff: %s>", err)
			}
			return fmt.Errorf(`Expected no changes, but changes were found:

%s


You may need to run:

	$ go run infra/bots/gen_tasks.go

`, diff)
		}
	} else {
		if err := ioutil.WriteFile(outFile, enc, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

// MustFinish validates and writes out the TasksCfg, or, if the --test flag is
// provided, verifies that the contents have not changed. Panics on failure.
func (b *TasksCfgBuilder) MustFinish() {
	if err := b.Finish(); err != nil {
		sklog.Error(err)
		os.Exit(1)
	}
}
