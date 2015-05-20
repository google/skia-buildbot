// packages is utilities for working with Debian packages and package lists.
package packages

import (
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
	"time"

	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/util"

	"github.com/skia-dev/glog"

	"code.google.com/p/google-api-go-client/storage/v1"
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
				Services: strings.Split(safeGet(o.Metadata, "services", ""), " "),
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
	if resp.StatusCode != 200 {
		return ret, fmt.Errorf("Wrong status code: %#v", *resp)
	}
	defer util.Close(resp.Body)
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

// AllInstalled returns a map of all known server names to their list of installed package names.
func AllInstalled(client *http.Client, store *storage.Service, names []string) (map[string]*Installed, error) {
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

// PutInstalled writes a new list of installed packages for the given server.
func PutInstalled(store *storage.Service, client *http.Client, serverName string, packages []string, generation int64) error {
	b, err := json.Marshal(packages)
	if err != nil {
		return fmt.Errorf("Failed to encode installed packages: %s", err)
	}
	buf := bytes.NewBuffer(b)
	req := store.Objects.Insert("skia-push", &storage.Object{Name: "server/" + serverName + ".json"}).Media(buf)
	if generation != -1 {
		req = req.IfGenerationMatch(generation)
	}
	if _, err = req.Do(); err != nil {
		return fmt.Errorf("Failed to write installed packages list to Google Storage for %s: %s", serverName, err)
	}
	return nil
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
