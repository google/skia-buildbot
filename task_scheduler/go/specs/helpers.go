package specs

/*
	Helper functions for client repos.
*/

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/common"
)

var (
	// Flags.
	test = flag.Bool("test", false, "Run in test mode: verify that the output hasn't changed.")
)

// getCheckoutRoot returns the path of the root of the checkout.
func getCheckoutRoot() (string, error) {
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
		s, err := os.Stat(path.Join(cwd, "infra", "bots"))
		if err == nil && s.IsDir() {
			return cwd, nil
		}
		// Check for .git dir.
		s, err = os.Stat(path.Join(cwd, ".git"))
		if err == nil && s.IsDir() {
			return cwd, nil
		}

		// Stop if we're at the filesystem root.
		if cwd == string(filepath.Separator) {
			return "", fmt.Errorf("Unable to find repository root.")
		}
		cwd = filepath.Clean(path.Join(cwd, ".."))
	}
}

// TasksCfgBuilder is a helper struct used for building a TasksCfg.
type TasksCfgBuilder struct {
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

	root, err := getCheckoutRoot()
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

// AddTask adds a TaskSpec to the TasksCfgBuilder. Returns an error if the
// config already contains a Task with the same name and a different
// implementation.
func (b *TasksCfgBuilder) AddTask(name string, t *TaskSpec) error {
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
	versionFile := path.Join(b.root, "infra", "bots", "assets", assetName, "VERSION")
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
	if assetName == "win_toolchain" {
		pkg.Path = "t" // Workaround for path length limit on Windows.
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
	// Validate the config.
	if err := b.cfg.Validate(); err != nil {
		return err
	}

	// Encode the JSON config.
	enc, err := json.MarshalIndent(b.cfg, "", "  ")
	if err != nil {
		return err
	}
	// The json package escapes HTML characters, which makes our output
	// much less readable. Replace the escape characters with the real
	// character.
	enc = bytes.Replace(enc, []byte("\\u003c"), []byte("<"), -1)

	// Add a newline to the end of the file. Most text editors add one, so
	// adding one here enables manual editing of the file, even though we'd
	// rather that not happen.
	enc = append(enc, []byte("\n")...)

	// Write the tasks.json file.
	outFile := path.Join(b.root, TASKS_CFG_FILE)
	if *test {
		// Don't write the file; read it and compare.
		expect, err := ioutil.ReadFile(outFile)
		if err != nil {
			return err
		}
		if !bytes.Equal(expect, enc) {
			return fmt.Errorf("Expected no changes, but changes were found!")
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
		sklog.Fatal(err)
	}
}
