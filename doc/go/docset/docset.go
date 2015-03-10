// docset keeps track of checkouts of a repository of Markdown documents.
package docset

/*

  DocSets work around a `workDir` which is presumed to be a directory with enough space
  to handle many checkouts of the Markdown repository. It has the following structure:

  {workDir}
    /primary/ - The primary checkout of the Markdown repository.
    /patches/
       {issue_id}-{patchset_id}/ - A patched checkout of the Markdown respository.

  Each repo should have a directory /site that contains all the documentation in Markdown
  and associated files such as PNG images, For example:

  site
  ├── dev
  │   ├── contrib
  │   │   ├── codereviews.md
  │   │   └── index.md
  │   └── index.md
  ├── index.md
  ├── logo.png
  ├── roles.md
  ├── roles.png
  ├── user
  │   ├── api.md
  │   ├── download.md
  │   ├── index.md
  │   ├── issue-tracker.md
  │   └── quick
  │       ├── index.md
  │       └── linux.md
  └── xtra
      └── index.md

*/

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/mail"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/doc/go/config"
	"go.skia.org/infra/doc/go/reitveld"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	MARKDOWN_CACHE_SIZE = 100
)

var (
	rc = reitveld.NewClient()
)

// DocSet is a single checked out repository of Markdown documents.
type DocSet struct {
	// repoDir is the directory the repo is checked out into.
	repoDir string
	// navigation is the HTML formatted navigation structure for the given repo.
	navigation string

	cache *lru.Cache

	mutex sync.Mutex
}

// newDocSet does the core of the work for both NewDocSet and NewDocSetForIssue.
//
// The repo is checked out into repoDir.
// If a valid issue and patchset are supplied then the repo will be patched with that CL.
// If refresh is true then the git repo will be periodically refreshed (git pull).
func newDocSet(repoDir, repo string, issue, patchset int64, refresh bool) (*DocSet, error) {
	if issue > 0 {
		issueInfo, err := rc.Issue(issue)
		if err != nil {
			err := fmt.Errorf("Failed to retrieve issue status %d: %s", issue, err)
			glog.Error(err)
			return nil, err
		}
		if issueInfo.Closed {
			return nil, fmt.Errorf("Issue %d is closed.", issue)
		}
	}
	git, err := gitinfo.CloneOrUpdate(repo, repoDir, false)
	if err != nil {
		glog.Fatalf("Failed to CloneOrUpdate repo %q: %s", repo, err)
	}
	if issue > 0 {
		cmd := exec.Command("patch", "-p1")
		cmd.StdinPipe()
		cmd.Dir = repoDir
		diff, err := rc.Patchset(issue, patchset)
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve patchset: %s", err)
		}
		cmd.Stdin = diff
		defer util.Close(diff)
		b, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("Error while patching %#v - %s: %s", *cmd, string(b), err)
		}
	}
	d := &DocSet{
		repoDir: repoDir,
	}
	d.BuildNavigation()
	if refresh {
		go func() {
			for _ = range time.Tick(config.REFRESH) {
				git.Update(true, false)
				d.BuildNavigation()
			}
		}()
	}
	return d, nil
}

// NewPreviewDocSet creates a new DocSet, one that is not refreshed.
func NewPreviewDocSet() (*DocSet, error) {
	// Start from cwd and move up until you find a .git file, then use that dir as repoDir.
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Can't find cwd: %s", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
			dir = path.Dir(dir)
		} else {
			break
		}
		if dir == "/" || dir == "." {
			return nil, fmt.Errorf("docserver --preview must be run from within the Git repo.")
		}
	}
	d := &DocSet{
		repoDir: dir,
	}
	d.BuildNavigation()
	d.cache = nil
	return d, nil
}

// NewDocSet creates a new DocSet, one that is periodically refreshed.
func NewDocSet(workDir, repo string) (*DocSet, error) {
	d, err := newDocSet(filepath.Join(workDir, "primary"), repo, -1, -1, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to CloneOrUpdate repo %q: %s", repo, err)
	}
	return d, nil
}

// NewDocSetForIssue creates a new DocSet patched to the latest patch level of
// the given issue.
//
// The returned DocSet is not periodically refreshed.
func NewDocSetForIssue(workDir, repo string, issue int64) (*DocSet, error) {
	// Only do pull and patch if directory doesn't exist.
	issueInfo, err := rc.Issue(issue)
	if err != nil {
		return nil, fmt.Errorf("Failed to load issue info: %s", err)
	}
	patchset := issueInfo.Patchsets[len(issueInfo.Patchsets)-1]
	addr, err := mail.ParseAddress(issueInfo.OwnerEmail)
	if err != nil {
		return nil, fmt.Errorf("CL contains invalid author email: %s", err)
	}
	domain := strings.Split(addr.Address, "@")[1]
	if !util.In(domain, config.WHITELIST) {
		return nil, fmt.Errorf("User is not authorized to test docset CLs.")
	}
	var d *DocSet
	repoDir := filepath.Join(workDir, "patches", fmt.Sprintf("%d-%d", issue, patchset))
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		d, err = newDocSet(repoDir, repo, issue, patchset, false)
		if err != nil {
			if err := os.RemoveAll(repoDir); err != nil {
				glog.Errorf("Failed to remove %q: %s", repoDir, err)
			}
			return nil, fmt.Errorf("Failed to create new doc set: %s", err)
		}
	} else {
		d = &DocSet{
			repoDir: repoDir,
		}
		d.BuildNavigation()
	}
	return d, nil
}

// RawFilename returns the absolute filename for the file associated with the
// given url.
//
// The bool returned will be true if the url identifies a raw resource, such as
// a PNG, as opposed to a Markdown file that should be processed.
func (d *DocSet) RawFilename(url string) (string, bool, error) {
	startFilename := filepath.Join(d.repoDir, config.REPO_SUBDIR, url)
	endFilename, err := findFile(startFilename)
	return endFilename, (startFilename == endFilename), err
}

// Body returns the contents of the given filename.
func (d *DocSet) Body(filename string) ([]byte, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	var err error = nil
	var body interface{} = nil
	ok := false
	if d.cache != nil {
		body, ok = d.cache.Get(filename)
	}
	if !ok {
		body, err = ioutil.ReadFile(filename)
		if err == nil {
			if d.cache != nil {
				d.cache.Add(filename, body)
			}
		}
	}
	return body.([]byte), err
}

// hasPrefix returns true if p is a prefix of a.
func hasPrefix(a, p []string) bool {
	if len(p) > len(a) {
		return false
	}
	for i, s := range p {
		if s != a[i] {
			return false
		}
	}
	return true
}

// diff determines how many levels of ul's we need to push and pop.
func diff(current, next []string) (int, int) {
	// Start by popping off values from the end of 'next' until we get a prefix
	// of 'current', which may be the empty list. Use that to calculate how many
	// </ul>'s we need to emit.
	end := 1
	for i := 0; i <= len(next); i++ {
		if hasPrefix(current, next[:len(next)-i]) {
			end = len(current) - (len(next) - i)
			break
		}
	}
	// If we are just adding a file in a new directory then don't end the list.
	if len(current) > 0 && len(next) > 0 && end == 1 && current[len(current)-1] == "" {
		end = 0
	}
	// We are always going to begin a new list.
	begin := 1
	// Unless we are adding in a file in the same directory, in which case do nothing.
	if len(current) > 0 && len(next) > 0 && end == 1 && current[len(current)-1] != "" && next[len(next)-1] != "" {
		end = 0
		begin = 0
	}

	return end, begin
}

// navTemplate is a self-refrential template used to recursively expand over node tree.
var navTemplate = template.Must(template.New("NODE").Parse(`
<li><a data-path="{{.Index.URL}}" href="{{.Index.URL}}">{{.Index.Name}}</a></li>
<ul class=files>
  {{range .Files}}
    <li><a data-path="{{.URL}}" href="{{.URL}}">{{.Name}}</a></li>
  {{end}}
</ul>
<ul class="dirs depth{{.Index.Depth}}">
  {{range .Dirs}}
    {{template "NODE" .}}
  {{end}}
</ul>`))

// nodeToHTML converts the node to HTML, keeping track of depth for pretty printing the output.
func nodeToHTML(n *node, depth int) string {
	b := &bytes.Buffer{}
	if err := navTemplate.Execute(b, n); err != nil {
		glog.Errorf("Failed to expand: %s", err)
		return ""
	}
	return b.String()
}

// metadata is the struct for deserializing JSON found in METADATA files.
type metadata struct {
	DirOrder  []string `json:"dirOrder"`
	FileOrder []string `json:"fileOrder"`
}

// buildNavString converts a slice of navEntry's into an HTML formatted
// navigation structure.
func buildNavString(n *node) string {
	return "\n<ul class=depth0>\n" + nodeToHTML(n, 1) + "</ul>\n"
}

// node is a single directory of site docs.
type node struct {
	Index navEntry
	Dirs  []*node
	Files []*navEntry
}

// nodeSlice is for sorting nodes.
type nodeSlice []*node

func (p nodeSlice) Len() int           { return len(p) }
func (p nodeSlice) Less(i, j int) bool { return p[i].Index.URL < p[j].Index.URL }
func (p nodeSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// walk the directory tree below root and populate a tree stucture of nodes.
func walk(root, path string) (*node, error) {
	ret := &node{
		Dirs:  []*node{},
		Files: []*navEntry{},
	}

	// for each directory fill in the navEntry for index.md
	rel, _ := filepath.Rel(root, path)
	rel = filepath.Clean("/" + rel)
	ret.Index = navEntry{
		URL:  rel,
		Name: readTitle(filepath.Join(path, "index.md"), rel),
	}
	m := &metadata{
		DirOrder:  []string{},
		FileOrder: []string{},
	}

	// populate all the other files
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	allFiles, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	for _, fi := range allFiles {
		// The contents are either files or directories.
		if fi.IsDir() {
			n, err := walk(root, filepath.Join(path, fi.Name()))
			if err != nil {
				return nil, err
			}
			ret.Dirs = append(ret.Dirs, n)
		} else if fi.Name() != "index.md" && strings.HasSuffix(fi.Name(), ".md") {
			fileRel := filepath.Clean(rel + "/" + fi.Name()[:len(fi.Name())-3])
			ret.Files = append(ret.Files, &navEntry{
				URL:  fileRel,
				Name: readTitle(filepath.Join(path, fi.Name()), fileRel),
			})
		} else if fi.Name() == "METADATA" {
			// Load JSON found in METADATA.
			metaPath := filepath.Join(path, fi.Name())
			f, err := os.Open(metaPath)
			if err != nil {
				glog.Warningf("Failed to open %q: %s", metaPath, err)
				continue
			}
			dec := json.NewDecoder(f)
			if err := dec.Decode(m); err != nil {
				glog.Warningf("Failed to decode %q: %s", metaPath, err)
			}
		}
	}

	// Sort dirs and files, use METADATA if available.
	// Pick out the matches in the order they appear, then sort the rest.
	// Yes, this is O(n^2), but for very small n.
	sortedDirs := []*node{}
	for _, name := range m.DirOrder {
		for i, n := range ret.Dirs {
			if name == filepath.Base(n.Index.URL) {
				sortedDirs = append(sortedDirs, n)
				ret.Dirs = append(ret.Dirs[:i], ret.Dirs[i+1:]...)
				break
			}
		}
	}
	sort.Sort(nodeSlice(ret.Dirs))
	ret.Dirs = append(sortedDirs, ret.Dirs...)

	sortedFiles := []*navEntry{}
	for _, name := range m.FileOrder {
		for i, n := range ret.Files {
			if name == filepath.Base(n.URL) {
				sortedFiles = append(sortedFiles, n)
				ret.Files = append(ret.Files[:i], ret.Files[i+1:]...)
				break
			}
		}
	}
	sort.Sort(navEntrySlice(ret.Files))
	ret.Files = append(sortedFiles, ret.Files...)

	return ret, nil
}

func printnode(n *node, depth int) {
	glog.Infof("Node: %*s%#v\n", depth*2, "", n.Index)
	for _, f := range n.Files {
		glog.Infof("File: %*s%#v\n", (depth+1)*2, "", *f)
	}
	for _, d := range n.Dirs {
		printnode(d, depth+1)
	}
}

func addDepth(n *node, depth int) {
	n.Index.Depth = depth
	for _, d := range n.Dirs {
		addDepth(d, depth+1)
	}
}

// BuildNavigation builds the Navigation for the DocSet.
func (d *DocSet) BuildNavigation() {
	// Walk the directory tree to build the navigation menu.
	root := filepath.Join(d.repoDir, config.REPO_SUBDIR)
	node, _ := walk(root, root)
	addDepth(node, 1)
	printnode(node, 0)
	s := buildNavString(node)
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.cache = lru.New(MARKDOWN_CACHE_SIZE)
	d.navigation = s
}

// Navigation returns the HTML formatted navigation.
func (d *DocSet) Navigation() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.navigation
}

// issueAndPatch is a regex for extracting the issue number from a directory name
// that is formatted like {issue_id}-{pathset_id}.
var issueAndPatch = regexp.MustCompile("([0-9]+)-[0-9]+")

// StartCleaner is a process that periodically checks the status of every issue
// that has been previewed and removes all the local files for closed issues.
func StartCleaner(workDir string) {
	glog.Info("Starting Cleaner")
	c := reitveld.NewClient()
	for _ = range time.Tick(config.REFRESH) {
		matches, err := filepath.Glob(workDir + "/patches/*")
		glog.Infof("Matches: %v", matches)
		if err != nil {
			glog.Errorf("Failed to retrieve list of patched checkouts: %s", err)
			continue
		}
		for _, filename := range matches {
			_, file := filepath.Split(filename)
			glog.Info(file)
			m := issueAndPatch.FindStringSubmatch(file)
			if len(m) < 2 {
				continue
			}
			issue, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil {
				glog.Errorf("Failed to parse %q as int: %s", m[1], err)
				continue
			}
			issueInfo, err := c.Issue(issue)
			if err != nil {
				glog.Errorf("Failed to retrieve issue status %d: %s", issue, err)
			}
			if issueInfo.Closed {
				if err := os.RemoveAll(filename); err != nil {
					glog.Errorf("Failed to remove %q: %s", filename, err)
				}
			}
		}
	}
}

// findFile takes a filename guess and turns into real name of a file.
//
// Look for the given file, if it exists then serve it raw with a guess at the
// content type. Otherwise append ".md" and return it as processed markdown.
// If it is a directory append "index.md" and return that.
func findFile(filename string) (string, error) {
	if stat, err := os.Stat(filename); err == nil {
		if stat.IsDir() {
			return findFile(filepath.Join(filename, "./index.md"))
		} else {
			return filename, nil
		}
	} else {
		if filepath.Ext(filename) == ".md" {
			return filename, err
		} else {
			return findFile(filename + ".md")
		}
	}
}

// pathOf returns a '/' terminated path for the given filename.
func pathOf(s string) string {
	if s[len(s)-1] == '/' {
		return s
	}
	parts := strings.Split(s, "/")
	if len(parts) > 0 {
		parts = parts[:len(parts)-1]
	}

	ret := strings.Join(parts, "/")
	if len(ret) == 0 || ret[len(ret)-1] != '/' {
		ret += "/"
	}
	return ret
}

// navEntry is a single directory entry in the Markdown repo.
type navEntry struct {
	Depth int
	URL   string
	Name  string
}

// navEntrySlice is a utility type for sorting navEntry's.
type navEntrySlice []*navEntry

func (p navEntrySlice) Len() int { return len(p) }
func (p navEntrySlice) Less(i, j int) bool {
	if pathOf(p[i].URL) < pathOf(p[j].URL) {
		return true
	}
	if pathOf(p[i].URL) == pathOf(p[j].URL) {
		return p[i].URL < p[j].URL
	}
	return false
}
func (p navEntrySlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// readTitle reads the first line from a Markdown file.
func readTitle(filename, def string) string {
	f, err := os.Open(filename)
	if err != nil {
		glog.Warningf("Failed to open file %s: %s", filename, err)
		return def
	}
	defer util.Close(f)
	reader := bufio.NewReader(f)
	title, err := reader.ReadString('\n')
	if err != nil {
		glog.Warningf("Failed to read title %s: %s", filename, err)
	}
	return strings.TrimSpace(title)
}
