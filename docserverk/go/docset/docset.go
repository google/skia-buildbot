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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/mail"
	"os"
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
	"go.skia.org/infra/docserverk/go/config"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MARKDOWN_CACHE_SIZE = 100
)

var (
	gc *gerrit.Gerrit

	IssueCommittedErr = errors.New("The requested issue is merged.")

	// markdownHeader matches the hashes that appear at the beginning of a
	// header.
	markdownHeader = regexp.MustCompile(`^#+\ `)
)

func Init(local bool) error {
	ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
	if err != nil {
		return err
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	gc, err = gerrit.NewGerrit(gerrit.GerritSkiaURL, client)
	return err
}

// DocSet is a single checked out repository of Markdown documents.
type DocSet struct {
	// repoDir is the directory the repo is checked out into.
	repoDir string
	// navigation is the HTML formatted navigation structure for the given repo.
	navigation string

	// A site map served to the Google crawler.
	siteMap string

	cache *lru.Cache

	mutex sync.Mutex
}

// newDocSet does the core of the work for both NewDocSet and NewDocSetForIssue.
//
// The repo is checked out somewhere under workDir.
// If a valid issue and patchset are supplied then the repo will be patched with that CL.
// If refresh is true then the git repo will be periodically refreshed (git pull).
func newDocSet(ctx context.Context, workDir, repo string, issue, patchset int64, refresh bool) (*DocSet, error) {
	primaryDir := filepath.Join(workDir, "primary")
	issueDir := filepath.Join(workDir, "patches", fmt.Sprintf("%d-%d", issue, patchset))
	repoDir := primaryDir
	if issue > 0 {
		repoDir = issueDir
		if _, err := os.Stat(issueDir); err == nil {
			d := &DocSet{
				repoDir: repoDir,
			}
			d.BuildNavigation()
			return d, nil
		}
	}

	if issue > 0 {
		info, err := gc.GetIssueProperties(ctx, issue)
		if err != nil {
			return nil, fmt.Errorf("Failed to load issue info: %s", err)
		}
		if info.Committed {
			return nil, IssueCommittedErr
		}
	}
	var gi *gitinfo.GitInfo
	var err error
	if issue > 0 {
		gi, err = gitinfo.CloneOrUpdate(ctx, primaryDir, repoDir, false)
	} else {
		gi, err = gitinfo.CloneOrUpdate(ctx, repo, repoDir, false)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to CloneOrUpdate repo %q: %s", repo, err)
	}

	if issue > 0 {
		// Run a git fetch for the branch where gerrit stores patches.
		//
		//  refs/changes/46/4546/1
		//                |  |   |
		//                |  |   +-> Patch set.
		//                |  |
		//                |  +-> Issue ID.
		//                |
		//                +-> Last two digits of Issue ID.

		issuePostfix := issue % 100
		output, err := git.GitDir(repoDir).Git(ctx, "fetch", repo, fmt.Sprintf("refs/changes/%02d/%d/%d", issuePostfix, issue, patchset))
		if err != nil {
			return nil, fmt.Errorf("Failed to execute Git %q: %s", output, err)
		}
		err = gi.Checkout(ctx, "FETCH_HEAD")
		if err != nil {
			return nil, fmt.Errorf("Failed to CloneOrUpdate repo %q: %s", repo, err)
		}
	}
	d := &DocSet{
		repoDir: repoDir,
	}
	d.BuildNavigation()
	if refresh {
		go func() {
			for range time.Tick(config.REFRESH) {
				util.LogErr(gi.Update(ctx, true, false))
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
func NewDocSet(ctx context.Context, workDir, repo string) (*DocSet, error) {
	return newDocSet(ctx, workDir, repo, -1, -1, true)
}

// NewDocSetForIssue creates a new DocSet patched to the latest patch level of
// the given issue.
//
// The returned DocSet is not periodically refreshed.
func NewDocSetForIssue(ctx context.Context, workDir, repo string, issue int64) (*DocSet, error) {
	info, err := gc.GetIssueProperties(ctx, issue)
	if err != nil {
		return nil, fmt.Errorf("Failed to load issue info: %s", err)
	}
	patchset := int64(len(info.Revisions))
	if patchset == 0 {
		return nil, fmt.Errorf("Failed to find a patchset for issue %d.", issue)
	}
	addr, err := mail.ParseAddress(info.Owner.Email)
	if err != nil {
		return nil, fmt.Errorf("CL contains invalid author email: %s", err)
	}
	domain := strings.Split(addr.Address, "@")[1]
	if !util.In(domain, config.ALLOWED_DOMAINS) {
		return nil, fmt.Errorf("User is not authorized to test docset CLs.")
	}
	return newDocSet(ctx, workDir, repo, issue, patchset, false)
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

// siteMapTemplate is a self-refrential template used to recursively expand over node tree.
var siteMapTemplate = template.Must(template.New("SITENODE").Parse(`https://skia.org{{.Index.URL}}
{{range .Files}}https://skia.org{{.URL}}
{{end}}{{range .Dirs}}{{template "SITENODE" .}}{{end}}`))

// nodeToSite converts the node to a sitemap.
func nodeToSite(n *node, depth int) string {
	b := &bytes.Buffer{}
	if err := siteMapTemplate.Execute(b, n); err != nil {
		sklog.Errorf("Failed to expand: %s", err)
		return ""
	}
	return b.String()
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

// buildSiteMapconverts a slice of navEntry's into an HTML formatted
// site map.
func buildSiteMap(n *node) string {
	return nodeToSite(n, 1)
}

// nodeToHTML converts the node to HTML, keeping track of depth for pretty printing the output.
func nodeToHTML(n *node, depth int) string {
	b := &bytes.Buffer{}
	if err := navTemplate.Execute(b, n); err != nil {
		sklog.Errorf("Failed to expand: %s", err)
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
				sklog.Warningf("Failed to open %q: %s", metaPath, err)
				continue
			}
			dec := json.NewDecoder(f)
			if err := dec.Decode(m); err != nil {
				sklog.Warningf("Failed to decode %q: %s", metaPath, err)
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
	sklog.Infof("Node: %*s%#v\n", depth*2, "", n.Index)
	for _, f := range n.Files {
		sklog.Infof("File: %*s%#v\n", (depth+1)*2, "", *f)
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
	s := buildNavString(node)
	sm := buildSiteMap(node)
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.cache = lru.New(MARKDOWN_CACHE_SIZE)
	d.navigation = s
	d.siteMap = sm
}

// Navigation returns the HTML formatted navigation.
func (d *DocSet) Navigation() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.navigation
}

// SiteMap returns the txt formatted site map.
func (d *DocSet) SiteMap() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.siteMap
}

// issueAndPatch is a regex for extracting the issue number from a directory name
// that is formatted like {issue_id}-{pathset_id}.
var issueAndPatch = regexp.MustCompile("([0-9]+)-[0-9]+")

// StartCleaner is a process that periodically checks the status of every issue
// that has been previewed and removes all the local files for closed issues.
func StartCleaner(workDir string) {
	sklog.Info("Starting Cleaner")
	for range time.Tick(config.REFRESH) {
		matches, err := filepath.Glob(workDir + "/patches/*")
		sklog.Infof("Matches: %v", matches)
		if err != nil {
			sklog.Errorf("Failed to retrieve list of patched checkouts: %s", err)
			continue
		}
		for _, filename := range matches {
			_, file := filepath.Split(filename)
			sklog.Info(file)
			m := issueAndPatch.FindStringSubmatch(file)
			if len(m) < 2 {
				continue
			}
			issue, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil {
				sklog.Errorf("Failed to parse %q as int: %s", m[1], err)
				continue
			}
			info, err := gc.GetIssueProperties(context.TODO(), issue)
			// Delete closed and missing issues.
			if err != nil || info.Committed {
				if err := os.RemoveAll(filename); err != nil {
					sklog.Errorf("Failed to remove %q: %s", filename, err)
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
		sklog.Warningf("Failed to open file %s: %s", filename, err)
		return def
	}
	defer util.Close(f)
	reader := bufio.NewReader(f)
	title, err := reader.ReadString('\n')
	if err != nil {
		sklog.Warningf("Failed to read title %s: %s", filename, err)
	}
	if strings.HasPrefix(title, "#") {
		title = markdownHeader.ReplaceAllString(title, "")
	}
	title = strings.TrimSpace(title)
	return title
}
