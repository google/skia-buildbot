// mirrors package brings up a verdaccio mirror for each supported project.
package mirrors

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"sync"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

const (
	verdaccioDirName        = "verdaccio"
	verdaccioStorageDirName = "storage"
	verdaccioLogFileName    = "verdaccio.log"
)

// VerdaccioMirror implements types.ProjectMirror.
type VerdaccioMirror struct {
	workDir             string
	verdaccioDir        string
	verdaccioConfigPath string
	verdaccioStorageDir string
	projectName         string
	publicURL           string

	// Maintains an in-memory map of download package tarballs.
	// This map is used to determine which packages are not
	// available on the mirror yet and will require an external
	// network call to download.
	downloadedPackageTarballs map[string]interface{}
	// Mutex that contains access to the above map.
	downloadedPackageTarballsMtx sync.RWMutex
}

// NewVerdaccioMirror returns an instance of VerdaccioMirror.
func NewVerdaccioMirror(projectName, workDir, host string, cfgTmpl *template.Template) (types.ProjectMirror, error) {
	// Create all necessary directories and files.
	verdaccioDir := path.Join(workDir, verdaccioDirName)
	if err := os.MkdirAll(verdaccioDir, 0755); err != nil {
		return nil, skerr.Wrapf(err, "Could not create %s", verdaccioDir)
	}
	// Create the verdaccio config file.
	verdaccioConfigPath, err := createCfgFile(projectName, workDir, verdaccioDir, cfgTmpl)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Verdaccio storage dir. This will be created automatically by verdaccio.
	verdaccioStorageDir := path.Join(verdaccioDir, verdaccioStorageDirName)

	// Populate the in-memory map of downloaded package tarballs.
	downloadedPackageTarballs, err := GetTarballsInMirrorStorage(verdaccioStorageDir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Find this mirror's publicURL.
	publicURL := fmt.Sprintf("%s/%s/", host, projectName)

	return &VerdaccioMirror{
		workDir:                   workDir,
		verdaccioDir:              verdaccioDir,
		verdaccioConfigPath:       verdaccioConfigPath,
		verdaccioStorageDir:       verdaccioStorageDir,
		projectName:               projectName,
		downloadedPackageTarballs: downloadedPackageTarballs,
		publicURL:                 publicURL,
	}, nil
}

// createCfgFile creates the verdaccio config file using the provided template. Returns
// the path to the config file.
func createCfgFile(projectName, workDir, verdaccioDir string, cfgTmpl *template.Template) (string, error) {
	verdaccioConfigFilePath := path.Join(verdaccioDir, "config.yaml")
	f, err := os.Create(verdaccioConfigFilePath)
	if err != nil {
		return "", skerr.Wrapf(err, "Could not create %s", verdaccioConfigFilePath)
	}
	defer f.Close()
	if err := cfgTmpl.Execute(f, map[string]string{
		"Path":        workDir,
		"ProjectName": projectName,
	}); err != nil {
		return "", skerr.Wrapf(err, "Could not execute template for %s", projectName)
	}

	return verdaccioConfigFilePath, nil
}

// GetProjectName implements the types.ProjectMirror interface.
func (m *VerdaccioMirror) GetProjectName() string {
	return m.projectName
}

// StartMirror implements the types.ProjectMirror interface.
func (m *VerdaccioMirror) StartMirror(ctx context.Context, port int) error {
	go func() {
		// Create the verdaccio log before starting verdaccio.
		verdaccioLogPath := path.Join(m.verdaccioDir, verdaccioLogFileName)
		verdaccioLogFile, err := os.Create(verdaccioLogPath)
		if err != nil {
			sklog.Fatalf("Could not create %s: %s", verdaccioLogFile, err)
		}
		defer verdaccioLogFile.Close()
		// Start verdaccio.
		m.startVerdaccioMirror(ctx, port, verdaccioLogFile)
	}()

	return nil
}

// startVerdaccioMirror brings up a running verdaccio mirror locally.
func (m *VerdaccioMirror) startVerdaccioMirror(ctx context.Context, port int, logFile *os.File) {
	verdaccioCmd := executil.CommandContext(ctx, "verdaccio", "--config="+m.verdaccioConfigPath, fmt.Sprintf("--listen=%d", port))
	verdaccioCmd.Dir = m.workDir
	verdaccioCmd.Stdout = logFile
	verdaccioCmd.Env = os.Environ()
	verdaccioCmd.Env = append(verdaccioCmd.Env,
		fmt.Sprintf("VERDACCIO_PUBLIC_URL=%s", m.publicURL),
		// Makes the logs more verbose and useful for debugging.
		"NODE_DEBUG=request",
		"DEBUG=express:*")
	sklog.Info(verdaccioCmd.String())
	if err := verdaccioCmd.Run(); err != nil {
		sklog.Fatalf("Could not start verdaccio in %s: %s", m.workDir, err)
	}
}

// AddToDownloadedPackageTarballs implements the types.ProjectMirror interface.
func (m *VerdaccioMirror) AddToDownloadedPackageTarballs(packageTarballName string) {
	m.downloadedPackageTarballsMtx.Lock()
	defer m.downloadedPackageTarballsMtx.Unlock()
	m.downloadedPackageTarballs[packageTarballName] = true
}

// IsPackageTarballDownloaded implements the types.ProjectMirror interface.
func (m *VerdaccioMirror) IsPackageTarballDownloaded(packageTarballName string) bool {
	m.downloadedPackageTarballsMtx.RLock()
	defer m.downloadedPackageTarballsMtx.RUnlock()
	_, downloaded := m.downloadedPackageTarballs[packageTarballName]
	return downloaded
}

// GetTarballsInMirrorStorage returns a map of the packages (including their versions)
// that are available locally on the mirror. These are the packages that the mirror
// does not have to hit the public NPM registry for.
func GetTarballsInMirrorStorage(verdaccioStorageDir string) (map[string]interface{}, error) {
	installedPackages := map[string]interface{}{}
	if _, err := os.Stat(verdaccioStorageDir); !os.IsNotExist(err) {
		err = filepath.Walk(verdaccioStorageDir, func(path string, f os.FileInfo, err error) error {
			if !f.IsDir() && filepath.Ext(path) == ".tgz" {
				installedPackages[f.Name()] = struct{}{}
			}
			return nil
		})
		if err != nil {
			return nil, skerr.Wrapf(err, "Could not look for packages in %s", verdaccioStorageDir)
		}
	}
	return installedPackages, nil
}
