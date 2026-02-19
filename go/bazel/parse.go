package bazel

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/go-python/gpython/ast"
	"github.com/go-python/gpython/parser"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// IsBazelFile returns true if the filename looks like a Bazel file.
func IsBazelFile(file string) bool {
	return strings.HasSuffix(file, "WORKSPACE") ||
		strings.HasSuffix(file, ".bazel") ||
		strings.HasSuffix(file, ".bzl")
}

// DependencyID represents the unique identifier for a dependency.
type DependencyID string

// Dependency represents a pin of one or more dependencies.
type Dependency interface {
	GetID() DependencyID
	GetVersion() string
	SetVersion(string)
	GetFunction() string
	Validate() error
}

// SingleDependency represents one dependency version pin.
type SingleDependency struct {
	ID         DependencyID
	Version    string
	versionPos *ast.Pos
	SHA256     string
	sha256Pos  *ast.Pos
	Function   string
}

func (d SingleDependency) GetID() DependencyID {
	return d.ID
}

func (d SingleDependency) GetVersion() string {
	return d.Version
}

func (d SingleDependency) SetVersion(ver string) {
	d.Version = ver
}

func (d SingleDependency) GetFunction() string {
	return d.Function
}

// Validate returns an error if the Dependency is not valid.
func (d SingleDependency) Validate() error {
	if d.ID == "" {
		return skerr.Fmt("ID is unset")
	}
	if d.Version == "" {
		return skerr.Fmt("Version is unset")
	}
	if d.versionPos == nil {
		return skerr.Fmt("versionPos is unset")
	}
	if d.SHA256 == "" {
		return skerr.Fmt("SHA256 is unset")
	}
	if d.sha256Pos == nil {
		return skerr.Fmt("sha256Pos is unset")
	}
	return nil
}

// Assert that SingleDependency implements Dependency.
var _ Dependency = &SingleDependency{}

// MetaDependency represents a pin of multiple related dependencies. These
// dependencies share a single version ID but have different SHA256 sums.
type MetaDependency struct {
	ID         DependencyID
	Version    string
	versionPos *ast.Pos
	SHA256     map[string]string
	sha256Pos  map[string]*ast.Pos
	Function   string
}

// Validate returns an error if the Dependency is not valid.
func (d MetaDependency) Validate() error {
	if d.ID == "" {
		return skerr.Fmt("ID is unset")
	}
	if d.Version == "" {
		return skerr.Fmt("Version is unset")
	}
	if d.versionPos == nil {
		return skerr.Fmt("versionPos is unset")
	}
	if len(d.SHA256) == 0 {
		return skerr.Fmt("SHA256 is unset")
	}
	if len(d.sha256Pos) != len(d.SHA256) {
		return skerr.Fmt("sha256Pos is invalid")
	}
	return nil
}

func (d MetaDependency) GetID() DependencyID {
	return d.ID
}

func (d MetaDependency) GetVersion() string {
	return d.Version
}

func (d MetaDependency) SetVersion(ver string) {
	d.Version = ver
}

func (d MetaDependency) GetFunction() string {
	return d.Function
}

// Assert that MetaDependency implements Dependency.
var _ Dependency = &MetaDependency{}

// GetDep parses the file contents and returns the given dependency.
func GetDep(content string, dep DependencyID) (Dependency, error) {
	entries, err := ParseDeps(content)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	entry, ok := entries[dep]
	if !ok {
		b, err := json.MarshalIndent(entries, "", "  ")
		if err == nil {
			return nil, skerr.Fmt("Cannot find item with id=%q from these entries:\n%s", dep, string(b))
		} else {
			return nil, skerr.Fmt("Unable to find %q! Failed to encode entries with: %s", dep, err)
		}
	}
	return entry, nil
}

func resolveDepType(dep Dependency) (*SingleDependency, *MetaDependency, error) {
	if singleDep, ok := dep.(*SingleDependency); ok {
		return singleDep, nil, nil
	}
	if metaDep, ok := dep.(*MetaDependency); ok {
		return nil, metaDep, nil
	}
	return nil, nil, skerr.Fmt("unknown dependency type %+v", dep)
}

// SetDep parses the file contents, updates the version of the given
// dependency, and returns the new file contents or any error that occurred.
func SetDep(content string, dep Dependency) (string, error) {
	if dep.GetVersion() == "" {
		return "", skerr.Fmt("version is required")
	}

	// Parse the file content.
	entries, err := ParseDeps(content)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Find the requested dependency.
	existingDep, ok := entries[dep.GetID()]
	if !ok {
		return "", skerr.Fmt("Failed to find dependency with id %q", dep.GetID())
	}

	singleDep, metaDep, err := resolveDepType(dep)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	existingSingleDep, existingMetaDep, err := resolveDepType(existingDep)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if singleDep != nil && existingSingleDep == nil {
		return "", skerr.Fmt("existing dependency in file is a MetaDependency but a SingleDependency was provided")
	}
	if singleDep == nil && existingSingleDep != nil {
		return "", skerr.Fmt("existing dependency in file is a SingleDependency but a MetaDependency was provided")
	}

	// Replace the old version with the new.
	lines := strings.Split(content, "\n")
	if singleDep != nil {
		replace(lines, existingSingleDep.versionPos, existingSingleDep.Version, singleDep.Version)
		replace(lines, existingSingleDep.sha256Pos, existingSingleDep.SHA256, singleDep.SHA256)
	} else {
		// Check that the old and new sha256 sums refer to the same deps.
		if !util.MapKeysEqual(metaDep.SHA256, existingMetaDep.SHA256) {
			oldKeys := util.Keys(existingMetaDep.SHA256)
			newKeys := util.Keys(metaDep.SHA256)
			sort.Strings(oldKeys)
			sort.Strings(newKeys)
			return "", skerr.Fmt("sha256 map keys don't match: %v vs %v", oldKeys, newKeys)
		}
		replace(lines, existingMetaDep.versionPos, existingMetaDep.Version, metaDep.Version)
		for id, pos := range existingMetaDep.sha256Pos {
			oldSha256, ok := existingMetaDep.SHA256[id]
			if !ok {
				return "", skerr.Fmt("missing sha256 for %s in existing dep", id)
			}
			newSha256, ok := metaDep.SHA256[id]
			if !ok {
				return "", skerr.Fmt("missing sha256 for %s in updated dep", id)
			}
			replace(lines, pos, oldSha256, newSha256)
		}
	}
	return strings.Join(lines, "\n"), nil
}

// replace a string in the given file content lines.
func replace(contentLines []string, pos *ast.Pos, old, new string) {
	lineIdx := pos.Lineno - 1 // Lineno starts at 1.
	line := contentLines[lineIdx]
	newLine := line[:pos.ColOffset] + strings.Replace(line[pos.ColOffset:], old, new, 1)
	contentLines[lineIdx] = newLine
}

// ParseDeps parses the file contents and returns all dependencies, keyed by ID.
func ParseDeps(content string) (map[DependencyID]Dependency, error) {
	parsed, err := parser.ParseString(content, "exec")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Loop through all top-level statements in the file, collecting
	// dependencies and the positions in the file where their versions are set.
	deps := map[DependencyID]Dependency{}
	for _, stmt := range parsed.(*ast.Module).Body {
		// The dependencies we're looking for are function calls, each of which
		// lives inside an ExprStmt. We ignore all other types of Stmt.
		//
		// For example:
		//
		// # This is a top-level expression statement, which is a function call:
		// container_pull(
		//     name = "empty_container",
		//     digest = "SHA256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
		//     registry = "index.docker.io",
		//     repository = "alpine",
		// )
		//
		// # This is a top-level assignment statement. It will be ignored:
		// PROTOC_BUILD_FILE_CONTENT = """
		// exports_files(["bin/protoc"], visibility = ["//visibility:public"])
		// """
		if stmt.Type().Name == ast.ExprStmtType.Name {
			exprStmt := stmt.(*ast.ExprStmt).Value
			if exprStmt.Type().Name == ast.CallType.Name {
				dep, err := parseCall(exprStmt.(*ast.Call))
				if err != nil {
					return nil, skerr.Wrap(err)
				}
				if dep == nil {
					continue
				}
				if err := dep.Validate(); err != nil {
					return nil, skerr.Wrap(err)
				}
				if _, ok := deps[dep.GetID()]; ok {
					return nil, skerr.Fmt("found multiple entries for %q", dep.GetID())
				}
				deps[dep.GetID()] = dep
			}
		}
	}
	return deps, nil
}

// parseCall parses a Call to return one or more dependencies, if any can be
// found.
func parseCall(call *ast.Call) (Dependency, error) {
	var funcName string
	fn, ok := call.Func.(*ast.Name)
	if ok {
		funcName = string(fn.Id)
	} else {
		attr, ok := call.Func.(*ast.Attribute)
		if !ok {
			return nil, skerr.Fmt("Unknown type for function call at line %d", call.Lineno)
		}
		obj, ok := attr.Value.(*ast.Name)
		if !ok {
			return nil, skerr.Fmt("Unknown type for attribute value on function call at line %d", call.Lineno)
		}
		funcName = string(obj.Id) + "." + string(attr.Attr)
	}
	parseFn := map[string]func(*ast.Call, string) (Dependency, error){
		"cipd_install":       parseCIPDPackage,
		"cipd.package":       parseCIPDPackage,
		"cipd.download_http": parseCIPDPackage,
		"cipd.download_cipd": parseCIPDPackage,
		"container_pull":     parseContainerPull,
		"oci.pull":           parseOCIPull,
	}[funcName]
	if parseFn != nil {
		dep, err := parseFn(call, funcName)
		if err != nil {
			return nil, skerr.Wrapf(err, "parsing %q call at line %d", funcName, call.Lineno)
		}
		return dep, nil
	}
	return nil, nil
}

// parseDepFromCall is a helper function used to extract a dependency from a
// Call using the given idKeyword and versionKeyword.
func parseCIPDPackage(call *ast.Call, funcName string) (Dependency, error) {
	id, _, err := getCallArgValueString(call, "cipd_package")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	version, versionPos, err := getCallArgValueString(call, "tag")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	platformToSha256, platformToSha256Pos, err := getCallArgValueDict(call, "platform_to_sha256")
	if err == nil && len(platformToSha256) > 0 {
		return &MetaDependency{
			ID:         DependencyID(id),
			Version:    version,
			versionPos: versionPos,
			SHA256:     platformToSha256,
			sha256Pos:  platformToSha256Pos,
			Function:   funcName,
		}, nil
	}

	sha256, sha256Pos, err := getCallArgValueString(call, "sha256")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &SingleDependency{
		ID:         DependencyID(id),
		Version:    version,
		versionPos: versionPos,
		SHA256:     sha256,
		sha256Pos:  sha256Pos,
		Function:   funcName,
	}, nil
}

// parseContainerPull parses a call to container_pull to return a dependency and
// the position where the version is defined.
func parseContainerPull(call *ast.Call, funcName string) (Dependency, error) {
	registry, _, err := getCallArgValueString(call, "registry")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	repository, _, err := getCallArgValueString(call, "repository")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	id := registry + "/" + repository
	digest, digestPos, err := getCallArgValueString(call, "digest")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &SingleDependency{
		ID:         DependencyID(id),
		Version:    digest,
		versionPos: digestPos,
		SHA256:     digest,
		sha256Pos:  digestPos,
		Function:   funcName,
	}, nil
}

// parseOCIPull parses a call to oci_pull to return a dependency and
// the position where the version is defined.
func parseOCIPull(call *ast.Call, funcName string) (Dependency, error) {
	repository, _, err := getCallArgValueString(call, "image")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	digest, digestPos, err := getCallArgValueString(call, "digest")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &SingleDependency{
		ID:         DependencyID(repository),
		Version:    digest,
		versionPos: digestPos,
		SHA256:     digest,
		sha256Pos:  digestPos,
		Function:   funcName,
	}, nil
}

// getCallArgValueString searches the keyword arguments of the call for one with
// the matching keyword. If found, it returns the string value of the argument,
// and the position of that string value. This only works for string literals.
func getCallArgValueString(call *ast.Call, keyword string) (string, *ast.Pos, error) {
	for _, kw := range call.Keywords {
		if string(kw.Arg) == keyword {
			if kw.Value.Type().Name == ast.StrType.Name {
				rv := string(kw.Value.(*ast.Str).S)
				if rv == "" {
					return "", nil, skerr.Fmt("found empty string for argument %q", keyword)
				}
				return rv, &ast.Pos{
					Lineno:    kw.Value.GetLineno(),
					ColOffset: kw.Value.GetColOffset(),
				}, nil
			}
		}
	}
	return "", nil, skerr.Fmt("no keyword argument %q found for call", keyword)
}

func getCallArgValueDict(call *ast.Call, keyword string) (map[string]string, map[string]*ast.Pos, error) {
	for _, kw := range call.Keywords {
		if string(kw.Arg) == keyword {
			if kw.Value.Type().Name == ast.DictType.Name {
				dict := kw.Value.(*ast.Dict)
				rv := make(map[string]string, len(dict.Keys))
				rvPos := make(map[string]*ast.Pos, len(dict.Keys))
				for idx, keyExpr := range dict.Keys {
					if keyExpr.Type().Name != ast.StrType.Name {
						return nil, nil, skerr.Fmt("invalid dict key type %q in %q call", keyExpr.Type().Name, call.Func.Type().Name)
					}
					valExpr := dict.Values[idx]
					if valExpr.Type().Name != ast.StrType.Name {
						return nil, nil, skerr.Fmt("invalid dict value type %q in %q call", valExpr.Type().Name, call.Func.Type().Name)
					}
					key := string(keyExpr.(*ast.Str).S)
					rv[key] = string(valExpr.(*ast.Str).S)
					rvPos[key] = &ast.Pos{
						Lineno:    valExpr.GetLineno(),
						ColOffset: valExpr.GetColOffset(),
					}
				}
				return rv, rvPos, nil
			} else {
				return nil, nil, skerr.Fmt("invalid type for keyword %q in %q call", keyword, call.Func.Type().Name)
			}
		}
	}
	return nil, nil, skerr.Fmt("no keyword argument %q found for call", keyword)
}
