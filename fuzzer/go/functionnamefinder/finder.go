package functionnamefinder

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/exec"
)

type Finder interface {
	FunctionName(packageName string, fileName string, lineNumber int) string
}

// asyncFinder is a wrapper around another Finder that will block until the other
// finder is loaded.
type asyncFinder struct {
	finder Finder
	lock   sync.Mutex
}

// FunctionName returns the function name associated with a packageName, fileName, and
// lineNumber.  If there is no function name found, common.UNKNOWN_FUNCTION is returned.
// This blocks until the underlying finder has been created.
func (a *asyncFinder) FunctionName(packageName string, fileName string, lineNumber int) string {
	a.lock.Lock()
	defer a.lock.Unlock()
	// if finder is nil, there was a problem creating the finder.
	if a.finder == nil {
		return common.UNKNOWN_FUNCTION
	}
	return a.finder.FunctionName(packageName, fileName, lineNumber)
}

// NewAsync asynchronously creates a FunctionNameFinder based on Skia source code.
// It first builds Skia using Clang's ast-dump flag and then parses the generated
// ast for method declarations.  Calls to FunctionName will block until the underlying
// finder is created.  Any errors creating it will be logged, rather than returned.
func NewAsync() Finder {
	m := asyncFinder{}
	m.lock.Lock()
	go func() {
		defer m.lock.Unlock()
		var err error
		m.finder, err = NewSync()
		if err != nil {
			glog.Errorf("Problem creating finder asynchronously: %s", err)
		}
	}()
	return &m
}

// NewSync synchronously creates a FunctionNameFinder based on Skia source code.
// It first builds Skia using Clang's ast-dump flag and then parses the generated
// ast for method declarations.
func NewSync() (Finder, error) {
	ast, err := buildSkiaAST()
	if err != nil {
		return nil, fmt.Errorf("Problem building AST: %s", err)
	}
	buf := bufio.NewScanner(bytes.NewBuffer(ast))
	glog.Infof("Parsing %d bytes", len(ast))
	return parseSkiaAST(buf)
}

// buildSkiaAST returns a ~13GB dump of all ASTs created when building Skia.
// It builds a release version of Skia using Clang, once without the -ast-dump flag
// and once with.  The output of the latter is returned.
func buildSkiaAST() ([]byte, error) {
	// TODO(kjlubick): Refactor this to share functionality with common.BuildClangDM?
	// clean previous build
	buildLocation := filepath.Join("out", "Release")
	if err := os.RemoveAll(filepath.Join(config.FrontEnd.SkiaRoot, buildLocation)); err != nil {
		return nil, err
	}

	gypCmd := &exec.Command{
		Name:      "./gyp_skia",
		Dir:       config.FrontEnd.SkiaRoot,
		LogStdout: false,
		LogStderr: false,
		Env: []string{
			`GYP_DEFINES=skia_clang_build=1`,
			fmt.Sprintf("CC=%s", config.Common.ClangPath),
			fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
		},
	}

	// run gyp
	if err := exec.Run(gypCmd); err != nil {
		glog.Errorf("Failed gyp: %s", err)
		return nil, err
	}
	ninjaPath := filepath.Join(config.Common.DepotToolsPath, "ninja")

	ninjaCmd := &exec.Command{
		Name:      ninjaPath,
		Args:      []string{"-C", buildLocation},
		LogStdout: true,
		LogStderr: true,
		Dir:       config.FrontEnd.SkiaRoot,
		Env: []string{
			fmt.Sprintf("CC=%s", config.Common.ClangPath),
			fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
		},
		InheritPath: true,
	}

	// first build
	// Skia needs to be built once without the -ast-dump flag before it is built
	// with -ast-dump, otherwise, the build fails with an error about deleting a file
	// that doesn't exist.
	if err := exec.Run(ninjaCmd); err != nil {
		glog.Errorf("Failed ninja: %s", err)
		return nil, err
	}

	// Quotes are NOT needed around the params.  Doing so actually causes
	// a failure that "-Xclang -ast-dump -fsyntax-only" is an unused argument.
	gypCmd.Env = append(gypCmd.Env, `CXXFLAGS=-Xclang -ast-dump -fsyntax-only`)

	// run gyp again to remake build files with ast-dump flags
	if err := exec.Run(gypCmd); err != nil {
		glog.Errorf("Failed gyp message: %s", err)
		return nil, err
	}

	// Run ninja again, which will dump the ast to std out
	var ast bytes.Buffer
	var stdErr bytes.Buffer

	ninjaCmd = &exec.Command{
		Name:      ninjaPath,
		Args:      []string{"-C", buildLocation},
		LogStdout: false,
		LogStderr: false,
		Stdout:    &ast,
		Stderr:    &stdErr,
		Dir:       config.FrontEnd.SkiaRoot,
		Env: []string{
			fmt.Sprintf("CC=%s", config.Common.ClangPath),
			fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
		},
		InheritPath: true,
	}
	glog.Info("Generating AST")

	if err := exec.Run(ninjaCmd); err != nil {
		return nil, fmt.Errorf("Error generating AST %s:\nstderr: %s", err, stdErr.String())
	}
	glog.Info("Done generating AST")

	return ast.Bytes(), nil
}

// file is a simple representation of a skia file.
type file struct {
	PackageName string
	FileName    string
}

// methodDeclaration represents where a method is declared and what it is called.
type methodDeclaration struct {
	StartLine int
	Name      string
}

type methodDeclarations []methodDeclaration

// Find returns the result of sort.Search and whether or not the element already exists
// in the slice.
func (m methodDeclarations) find(startLine int, name string) (int, bool) {
	i := sort.Search(len(m), func(i int) bool { return m[i].StartLine >= startLine })
	// Test to see if it is in the list
	if !(i < len(m) && m[i].Name == name && m[i].StartLine == startLine) {
		return i, false
	}
	return i, true
}

type methodLookupMap map[file]methodDeclarations

// Put inserts a relationship between the passed in file and methodDeclaration if such
// relationship is not already in the map, keeping all slices sorted.
func (m methodLookupMap) Put(f file, decl methodDeclaration) {
	methods := m[f]
	if methods == nil {
		m[f] = methodDeclarations{decl}
		return
	}
	if i, found := methods.find(decl.StartLine, decl.Name); found == false {
		// insert decl at index i to keep the list sorted
		methods = append(methods, decl)
		copy(methods[i+1:], methods[i:])
		methods[i] = decl
		m[f] = methods
	}
}

// FunctionName returns the function name associated with a packageName, fileName and
// line number.  If there is no function name found, common.UNKNOWN_FUNCTION is returned.
func (m methodLookupMap) FunctionName(packageName string, fileName string, lineNumber int) string {
	methods := m[file{packageName, fileName}]
	if methods == nil || len(methods) == 0 {
		return common.UNKNOWN_FUNCTION
	}

	if i, _ := methods.find(lineNumber, ""); i < len(methods) {
		// Check for an exact match of the line number
		if methods[i].StartLine == lineNumber {
			return methods[i].Name
		} else if i == 0 {
			return common.UNKNOWN_FUNCTION
		}
		// Look back one index for the function we want
		return methods[i-1].Name
	}
	return methods[len(methods)-1].Name
}

// matches something like:
// |-ClassTemplateDecl 0x3319ff0 <../../include/core/SkSize.h:13:1, line:61:1> line:13:30 SkTSize
// By looking for "|-" at the beginning of the line, which signals a new branch in the tree
// and "../../" which signals a change of file
var newFileMatch = regexp.MustCompile(`^\|-(\w+) [^<]+<\.\./\.\./([^:]+).*`)

// parseSkiaAST reads in the Skia ASTs supplied with the passed in scanner and
// returns a methodLookupMap based on the asts.
// First, it deduplicates the ASTs, as C++ will include and compile a given file 10s or hundreds
// of times while building.  This re-inclusion causes duplicate ASTs.
// After deduplicating the ASTs, it scans line by line for method declarations, putting
// found declarations in a map, which is returned.
func parseSkiaAST(astScanner *bufio.Scanner) (methodLookupMap, error) {
	glog.Info("Trimming ASTs")
	astFiles, err := trimASTs(astScanner)
	madeMap := make(methodLookupMap)
	if err != nil {
		return madeMap, err
	}

	glog.Infof("Parsing %d ASTs", len(astFiles))
	for f, contents := range astFiles {
		scan := bufio.NewScanner(bytes.NewReader([]byte(contents)))
		for scan.Scan() {
			s := scan.Text()
			decl := parseMethodDeclaration(s)
			if decl != nil {
				madeMap.Put(f, *decl)
			}
		}
	}
	return madeMap, nil
}

// trimASTs reads in the one large AST dump and returns a map of file->AST, one for each file
// that was in the original AST.
func trimASTs(astScanner *bufio.Scanner) (map[file]string, error) {
	astFiles := make(map[file]string)

	currFile := file{"not", "found"}
	var currBuff bytes.Buffer
	ignoreSection := false
	i := 0
	for astScanner.Scan() {
		if i++; i%100000 == 0 {
			fmt.Print(".")
		}
		s := astScanner.Text()
		if match := newFileMatch.FindStringSubmatch(s); match != nil {
			// match is []string, with index 0 being the whole line, \1 being the ast node
			// (which we don't want to be LinkageSpecDecl) and \2 being the class name.
			astFiles[currFile] = currBuff.String()
			currBuff.Reset()

			if match[1] == "LinkageSpecDecl" {
				// don't parse anything until the next top level branch
				ignoreSection = true
			} else if currFile = makeFile(match[2]); astFiles[currFile] != "" {
				// skip because we have already read the file in
				ignoreSection = true
			} else {
				ignoreSection = false
			}
		}

		if !ignoreSection {
			if _, err := currBuff.WriteString(s); err != nil {
				return astFiles, err
			}
			if _, err := currBuff.WriteRune('\n'); err != nil {
				return astFiles, err
			}
		}
	}

	astFiles[currFile] = currBuff.String()
	currBuff.Reset()
	return astFiles, nil
}

// makeFile splits up a string like "/alpha/beta/gamma/delta.h" into its package and file name
// e.g. file{"/alpha/beta/gamma/", "delta.h"}
func makeFile(s string) file {
	parts := strings.Split(s, "/")
	pack := strings.Join(parts[:len(parts)-1], "/") + "/"
	return file{
		PackageName: pack,
		FileName:    parts[len(parts)-1],
	}
}

// Look for CXXMethodDecl ... myMethodName 'all of the types and args'
var methodDeclarationMatch = regexp.MustCompile(`.*-CXXMethodDecl.+<line:(\d+).+?(\S+) '(.*)'`)
var constructorDeclarationMatch = regexp.MustCompile(`.*-CXXConstructorDecl.+<line:(\d+).+?(\S+) '(.*)'`)

// Used to find (and remove) argument modifiers
var keywords = regexp.MustCompile(`(const |struct |class |enum )+`)

// parseMethodDeclaration tries to find a methodDeclaration in the passed in string.
// If one exists, its name and line number at which it is declared is used to create a
// methodDeclaration, which is returned.  Otherwise, nil is returned.
func parseMethodDeclaration(s string) *methodDeclaration {
	match := methodDeclarationMatch.FindStringSubmatch(s)
	if match == nil {
		match = constructorDeclarationMatch.FindStringSubmatch(s)
	}
	if match == nil {
		return nil
	}
	// match is []string, index 1 is the line number, index 2 is the method name,
	// index 3 is the type string,
	n, err := strconv.ParseInt(match[1], 10, 32)
	if err != nil {
		glog.Errorf("Cannot parse line number from %s", match[1])
		return nil
	}
	// match 3 looks something like "_Bool (const void *, size_t) const"
	// We want the args on the inside of the params
	args := match[3]
	i := strings.Index(args, "(")
	j := strings.LastIndex(args, ")")
	args = keywords.ReplaceAllString(args[i:j+1], "")

	return &methodDeclaration{
		StartLine: int(n),
		Name:      match[2] + args,
	}
}
