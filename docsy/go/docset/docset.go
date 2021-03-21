// docset keeps track of checkouts of a repository of Markdown documents.
package docset

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	IssueCommittedErr = errors.New("The requested issue is merged.")
)

// Issue is the identifier for a CodeReview issue.
type Issue string

// entry in the docSet FileSystem cache.
type entry struct {
	patchset string

	// FileSystem of the rendered contents of the documentation.
	fs http.FileSystem

	// The last time we checked if patchset is the latest.
	lastPatchsetCheck time.Time
}

type CodeReview interface {
	// Returns the identifier used to signal the use of the main branch and not
	// an actual Issue.
	MainIssue() Issue

	IsIssueValid(ctx context.Context, issue Issue) bool

	ListFiles(ctx context.Context) ([]string, error)

	GetFile(ctx context.Context, filename string) (io.ReadCloser, error)
}

// gerritCodeView implements CodeReview.
type gerritCodeReview struct {
	gc *gerrit.Gerrit
}

func NewGerritCodeReview(local bool, gerritURL string) (*gerritCodeReview, error) {
	ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	gc, err := gerrit.NewGerrit(gerritURL, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gerritCodeReview{
		gc: gc,
	}, nil
}

func (cr *gerritCodeReview) MainIssue() Issue {
	return "-1"
}

const searchLimit = 10000

func (cr *gerritCodeReview) ListModifiedFiles(ctx context.Context, issue Issue) ([]string, error) {
	ci, err := cr.gc.GetChangeWithFiles(ctx, string(issue))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	allFiles := util.NewStringSet()
	for _, rev := range ci.Patchsets {
		for filename, _ := range rev.Files {
			allFiles[filename] = true
		}
	}
	return allFiles.Keys(), nil
}

func (cr *gerritCodeReview) GetFile(ctx context.Context, issue Issue, filename string) (io.ReadCloser, error) {
	return nil, nil
}

func (cr *gerritCodeReview) IsIssueValid(ctx context.Context, issue Issue) bool {
	if issue == cr.MainIssue() {
		return false
	}
	issueAsInt64, err := strconv.ParseInt(string(issue), 10, 64)
	if err != nil {
		return false
	}
	info, err := cr.gc.GetIssueProperties(ctx, issueAsInt64)
	if err != nil {
		sklog.Errorf("Failed to load issue info: %s", err)
		return false
	}
	if info.Committed {
		return false
	}
	return true
}

// Assert gerritCodeReview implements the CodeReview interface.
var _ CodeReview = (*gerritCodeReview)(nil)

// DocSet represents a set of hugo rendered sets of documentation.
//
// The DocSet will run a background tasks that monitors issues and updates the
// patchsets as they progress, and removes issues as they are closed.
type DocSet interface {
	// FileSystem returns a FileSystem of the rendered contents of the
	// documentation. Pass in the invalidIssue to get the main branch.
	FileSystem(ctx context.Context, issue Issue) (http.FileSystem, error)

	// Start the background processes.
	Start() error
}

const (
	contentSubDirectory     = "content"
	destinationSubDirectory = "destination"
)

// docSet implements DocSet.
//
// This implementation of DocSet works around a `workDir` which is presumed to
// be a directory with enough space to handle many checkouts of the
// documentation. It has the following structure:
//
//  {workDir}
//    /content/{issue}/ - A patched checkout of the documentation in the
//      docPath of the respository, with /-1/ representing the main branch at HEAD.
//    /destination/{issue}/ - The docsy rendered version of /content/{issue}.
//
// The background processes will monitor all current issues and update them to
// more recent patchsets periodically and also remove both /content/{issue}/ and
// /destination/{issue}/ directories once an issue is closed.
type docSet struct {

	// The path in the git repo where the docs are stored, e.g. "site" for Skia.
	docPath string

	// The directory containing docsy. See https://docsy.dev.
	docsyDir string

	// wordDir is the directory where the repos are checked out into.
	workDir string

	// The URL of the repo passed to 'git clone'.
	repoURL string

	// mutex protects access to cache.
	mutex sync.Mutex

	codeReview CodeReview

	// cache of rendered sets of documentation including the main set of docs
	// from HEAD at [InvalidIssue].
	cache map[Issue]*entry
}

func New(ctx context.Context, local bool, workDir string, docPath string, docsyDir string, repoURL string, codeReview CodeReview) (*docSet, error) {
	ret := &docSet{
		codeReview: codeReview,
		docPath:    docPath,
		docsyDir:   docsyDir,
		workDir:    workDir,
		repoURL:    repoURL,
		cache:      map[Issue]*entry{},
	}
	// Don't return until we're ready to serve the main documents at HEAD.
	if _, err := ret.FileSystem(ctx, codeReview.MainIssue()); err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// FileSystem implements DocSet.
func (d *docSet) FileSystem(ctx context.Context, issue Issue) (http.FileSystem, error) {
	d.mutex.Lock()
	if entry, ok := d.cache[issue]; ok {
		d.mutex.Unlock()
		return entry.fs, nil
	}
	defer d.mutex.Unlock()

	// From here on out we know that we haven't pulled the requested issue, so
	// we need to do that work.

	// First make sure the issue is valid and active.
	if !d.codeReview.IsIssueValid(ctx, issue) {
		return nil, IssueCommittedErr
	}

	// First either clone the repo, in the case of issue == -1, or copy the
	// docPath from -1 into /content/{issue}/ and then patch using Gerrit.
	contentDir := filepath.Join(d.workDir, contentSubDirectory, string(issue))
	if issue == d.codeReview.MainIssue() {
		_, err := gitinfo.CloneOrUpdate(ctx, d.repoURL, contentDir, false)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	} else {
		// Copy files over from the main branch.
		src := filepath.Join(d.workDir, contentSubDirectory, string(d.codeReview.MainIssue()), d.docPath)
		dst := filepath.Join(d.workDir, contentSubDirectory, string(issue), d.docPath)
		copyFilesAsLinks(src, dst)

		// Then download the patched files from the code review system.
		files, err := d.codeReview.ListFiles(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for _, filename := range files {
			r, err := d.codeReview.GetFile(ctx, filename)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			err = util.WithWriteFile(filepath.Join(dst, filename), func(w io.Writer) error {
				_, err := io.Copy(w, r)
				if err != nil {
					return skerr.Wrap(err)
				}
				return nil
			})
			if err != nil {
				return nil, skerr.Wrap(err)
			}
		}
	}

	// Then run Hugo/Docsy over the files to create the rendered tree.

	// The return an http.Dir on the Docsy output.
}

func copyFilesAsLinks(src, dst string) error {
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrap(err)
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, path), 0755)
		}
		return os.Symlink(filepath.Join(src, path), filepath.Join(dst, path))
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Start implements DocSet.
func (d *docSet) Start() error {
	return nil
}

/*

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
*/
