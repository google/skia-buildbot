// packages is utilities for working with Debian packages and package lists.
package packages

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	iexec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/storage/v1"
)

// Package represents a single Debian package uploaded to Google Storage.
type Package struct {
	Name     string // The unique name of this release package.
	Hash     string
	UserID   string
	Built    time.Time
	Dirty    bool
	Note     string
	Services []string
}

// Installed is a list of all the packages installed on a server.
//
type Installed struct {
	// Names is a list of package names, of the form "{appname}/{appname}:{author}:{date}:{githash}.deb"
	Names []string

	// Generation is the Google Storage generation number of the config file at the time we read it.
	// Use this to avoid the lost-update problem: https://cloud.google.com/storage/docs/generations-preconditions#_ReadModWrite
	Generation int64
}

// AllInfo keeps a cache of all installed packages and all available to be installed packages.
type AllInfo struct {
	mutex        sync.Mutex
	allInstalled map[string]*Installed
	allAvailable map[string][]*Package
	client       *http.Client
	store        *storage.Service
	serverNames  []string
}

func NewAllInfo(client *http.Client, store *storage.Service, serverNames []string) (*AllInfo, error) {
	a := &AllInfo{
		client:      client,
		store:       store,
		serverNames: serverNames,
	}
	if err := a.step(); err != nil {
		return nil, fmt.Errorf("Failed to create packages.AllInfo: %s", err)
	}
	go func() {
		for _ = range time.Tick(1 * time.Minute) {
			if err := a.step(); err != nil {
				glog.Errorf("Failed to update AllInfo: %s", err)
			}
		}
	}()
	return a, nil
}

func (a *AllInfo) step() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	var err error
	a.allInstalled, err = allInstalled(a.client, a.store, a.serverNames)
	if err != nil {
		return err
	}
	a.allAvailable, err = AllAvailable(a.store)
	if err != nil {
		return err
	}
	return nil
}

func (a *AllInfo) AllAvailable() map[string][]*Package {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.allAvailable
}

// AllAvailableByPackageName returns all known packages for all applications
// uploaded to gs://skia-push/debs/. They are mapped by the package name.
func (a *AllInfo) AllAvailableByPackageName() map[string]*Package {
	allAvailable := a.AllAvailable()
	ret := map[string]*Package{}
	for _, ps := range allAvailable {
		for _, p := range ps {
			ret[p.Name] = p
		}
	}
	return ret
}

// PutInstalled writes a new list of installed packages for the given server.
func (a *AllInfo) PutInstalled(serverName string, packages []string, generation int64) error {
	b, err := json.Marshal(packages)
	if err != nil {
		return fmt.Errorf("Failed to encode installed packages: %s", err)
	}
	buf := bytes.NewBuffer(b)
	req := a.store.Objects.Insert("skia-push", &storage.Object{Name: "server/" + serverName + ".json"}).Media(buf)
	if generation != -1 {
		req = req.IfGenerationMatch(generation)
	}
	if _, err = req.Do(); err != nil {
		return fmt.Errorf("Failed to write installed packages list to Google Storage for %s: %s", serverName, err)
	}
	return a.step()
}

func (a *AllInfo) AllInstalled() map[string]*Installed {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.allInstalled
}

// allInstalled returns a map of all known server names to their list of installed package names.
func allInstalled(client *http.Client, store *storage.Service, names []string) (map[string]*Installed, error) {
	ret := map[string]*Installed{}
	for _, name := range names {
		p, err := InstalledForServer(client, store, name)
		if err != nil {
			glog.Errorf("Failed to retrieve remote package list: %s", err)
		}
		ret[name] = p
	}
	return ret, nil
}

func safeGetTime(m map[string]string, key string) time.Time {
	value := safeGet(m, key, "")
	if value == "" {
		return time.Time{}
	}
	ret, err := time.Parse("2006-01-02T15:04:05Z", value)
	if err != nil {
		glog.Errorf("Failed to parse metadata datatime %s: %s", value, err)
	}
	return ret
}

func safeGetBool(m map[string]string, key string) bool {
	if safeGet(m, key, "false") == "true" {
		return true
	}
	return false
}

func safeGet(m map[string]string, key string, def string) string {
	if value, ok := m[key]; ok {
		return value
	} else {
		return def
	}
}

func safeGetStringSlice(m map[string]string, key string) []string {
	if value, ok := m[key]; !ok {
		return []string{}
	} else {
		value = strings.TrimSpace(value)
		if value == "" {
			return []string{}
		} else {
			return strings.Split(value, " ")
		}
	}
}

// PackageSlice is for sorting Packages by Built time.
type PackageSlice []*Package

func (p PackageSlice) Len() int           { return len(p) }
func (p PackageSlice) Less(i, j int) bool { return p[i].Built.After(p[j].Built) }
func (p PackageSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// AllAvailable returns all known packages for all applications uploaded to
// gs://skia-push/debs/.
func AllAvailable(store *storage.Service) (map[string][]*Package, error) {
	req := store.Objects.List("skia-push").Prefix("debs")
	ret := map[string][]*Package{}
	for {
		objs, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("Failed to list debian packages in Google Storage: %s", err)
		}
		for _, o := range objs.Items {
			key := safeGet(o.Metadata, "appname", "")
			if key == "" {
				glog.Errorf("Debian package without proper metadata: %s", o.Name)
				continue
			}
			p := &Package{
				Name:     o.Name[5:], // Strip of debs/ from the beginning.
				Hash:     safeGet(o.Metadata, "hash", ""),
				UserID:   safeGet(o.Metadata, "userid", ""),
				Built:    safeGetTime(o.Metadata, "datetime"),
				Dirty:    safeGetBool(o.Metadata, "dirty"),
				Note:     safeGet(o.Metadata, "note", ""),
				Services: safeGetStringSlice(o.Metadata, "services"),
			}
			if _, ok := ret[key]; !ok {
				ret[key] = []*Package{}
			}
			ret[key] = append(ret[key], p)
		}
		if objs.NextPageToken == "" {
			break
		}
		req.PageToken(objs.NextPageToken)
	}
	for _, value := range ret {
		sort.Sort(PackageSlice(value))
	}
	return ret, nil
}

// AllAvailableByPackageName returns all known packages for all applications
// uploaded to gs://skia-push/debs/. They are mapped by the package name.
func AllAvailableByPackageName(store *storage.Service) (map[string]*Package, error) {
	allAvailable, err := AllAvailable(store)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve all available packages: %s", err)
	}
	ret := map[string]*Package{}
	for _, ps := range allAvailable {
		for _, p := range ps {
			ret[p.Name] = p
		}
	}
	return ret, nil
}

// InstalledForServer returns a list of package names of installed packages for
// the given server.
func InstalledForServer(client *http.Client, store *storage.Service, serverName string) (*Installed, error) {
	ret := &Installed{
		Names:      []string{},
		Generation: -1,
	}

	filename := "server/" + serverName + ".json"
	obj, err := store.Objects.Get("skia-push", filename).Do()
	if err != nil {
		return ret, fmt.Errorf("Failed to retrieve Google Storage metadata about packages file %q: %s", filename, err)
	}

	glog.Infof("Fetching: %s", obj.MediaLink)
	req, err := gs.RequestForStorageURL(obj.MediaLink)
	if err != nil {
		return ret, fmt.Errorf("Failed to construct request object for media: %s", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ret, fmt.Errorf("Failed to retrieve packages file: %s", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return ret, fmt.Errorf("Wrong status code: %#v", *resp)
	}
	dec := json.NewDecoder(resp.Body)

	value := []string{}
	if err := dec.Decode(&value); err != nil {
		return ret, fmt.Errorf("Failed to decode packages file: %s", err)
	}
	sort.Strings(value)
	ret.Names = value
	ret.Generation = obj.Generation

	return ret, nil
}

// FromLocalFile loads a list of installed debian package names from a local file.
func FromLocalFile(filename string) ([]string, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, nil
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open file %s: %s", filename, err)
	}
	defer util.Close(f)

	dec := json.NewDecoder(f)
	value := []string{}
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("Failed to decode packages file: %s", err)
	}
	return value, nil
}

// ToLocalFile writes a list of debian packages to a local file.
func ToLocalFile(packages []string, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Failed to create file %s: %s", filename, err)
	}
	defer util.Close(f)

	enc := json.NewEncoder(f)
	if err := enc.Encode(packages); err != nil {
		return fmt.Errorf("Failed to write %s: %s", filename, err)
	}
	return nil
}

// Install downloads and installs a debian package from Google Storage.
func Install(client *http.Client, store *storage.Service, name string) error {
	glog.Infof("Installing: %s", name)
	obj, err := store.Objects.Get("skia-push", "debs/"+name).Do()
	if err != nil {
		return fmt.Errorf("Failed to retrieve Google Storage metadata about debian package: %s", err)
	}
	req, err := gs.RequestForStorageURL(obj.MediaLink)
	if err != nil {
		return fmt.Errorf("Failed to construct request object for media: %s", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to retrieve packages file: %s", err)
	}
	defer util.Close(resp.Body)
	f, err := ioutil.TempFile("", "skia-pull")
	if err != nil {
		return fmt.Errorf("Failed to create tmp file: %s", err)
	}
	_, copyErr := io.Copy(f, resp.Body)
	if err := f.Close(); err != nil {
		return fmt.Errorf("Failed to close temporary file: %v", err)
	}
	if copyErr != nil {
		return fmt.Errorf("Failed to download file: %s", copyErr)
	}

	if err := installDependencies(f.Name()); err != nil {
		return fmt.Errorf("Error installing dependencies: %s", err)
	}

	cmd := exec.Command("sudo", "dpkg", "-i", f.Name())
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		glog.Errorf("Install package stdout: %s", out.String())
		return fmt.Errorf("Failed to install package: %s", err)
	}
	glog.Infof("Install package stdout: %s", out.String())
	return nil
}

// getDependencies returns the value of the 'Depends' field in the control
// file of the given package.
func getDependencies(packageName string) (string, error) {
	const DEPENDS_PREFIX = "Depends:"

	output, err := iexec.RunSimple(fmt.Sprintf("dpkg -I %s", packageName))
	if err != nil {
		return "", err
	}
	glog.Infof("Got output for %s :\n\n%s", packageName, output)

	buf := bytes.NewBuffer([]byte(output))
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, DEPENDS_PREFIX) {
			return strings.TrimSpace(strings.TrimPrefix(line, DEPENDS_PREFIX)), nil
		}
	}
	return "", nil
}

// installDependencies installs all dependencies that are named in the
// 'Depends' field of the control file via apt-get.
func installDependencies(packageFileName string) error {
	dependencies, err := getDependencies(packageFileName)
	if err != nil {
		return fmt.Errorf("Error getting dependencies for %s: %s", packageFileName, err)
	}

	if dependencies != "" {
		if output, err := iexec.RunSimple("sudo apt-get update"); err != nil {
			return fmt.Errorf("Unable to update package cache Got error  %s\n\n and output: %s\n\n", err, output)
		}

		glog.Infof("Installing via apt-get: %s", dependencies)
		if output, err := iexec.RunSimple(fmt.Sprintf("sudo apt-get -y install %s", dependencies)); err != nil {
			return fmt.Errorf("Unable to install dependencies for %s. Got error: \n %s \n\n and output:\n\n%s", packageFileName, err, output)
		}
	} else {
		glog.Infof("No deps found.")
	}
	return nil
}
