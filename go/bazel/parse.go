package bazel

import (
	"encoding/json"
	"strings"

	"github.com/go-python/gpython/ast"
	"github.com/go-python/gpython/parser"
	"go.skia.org/infra/go/skerr"
)

// IsBazelFile returns true if the filename looks like a Bazel file.
func IsBazelFile(file string) bool {
	return strings.HasSuffix(file, "WORKSPACE") ||
		strings.HasSuffix(file, ".bazel") ||
		strings.HasSuffix(file, ".bzl")
}

// DependencyID represents the unique identifier for a dependency.
type DependencyID string

// Dependency represents one dependency version pin.
type Dependency struct {
	ID         DependencyID
	Version    string
	versionPos *ast.Pos
	SHA256     string
	sha256Pos  *ast.Pos
}

// Validate returns an error if the Dependency is not valid.
func (d Dependency) Validate() error {
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

// GetDep parses the file contents and returns the given dependency.
func GetDep(content string, dep DependencyID) (Dependency, error) {
	entries, err := ParseDeps(content)
	if err != nil {
		return Dependency{}, skerr.Wrap(err)
	}
	entry, ok := entries[dep]
	if !ok {
		b, err := json.MarshalIndent(entries, "", "  ")
		if err == nil {
			return Dependency{}, skerr.Fmt("Cannot find item with id=%q from these entries:\n%s", dep, string(b))
		} else {
			return Dependency{}, skerr.Fmt("Unable to find %q! Failed to encode entries with: %s", dep, err)
		}
	}
	return entry, nil
}

// SetDep parses the file contents, updates the version of the given
// dependency, and returns the new file contents or any error that occurred.
func SetDep(content string, id DependencyID, version, sha256 string) (string, error) {
	if version == "" || sha256 == "" {
		return "", skerr.Fmt("version and sha256 are required")
	}

	// Parse the file content.
	entries, err := ParseDeps(content)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Find the requested dependency.
	dep, ok := entries[id]
	if !ok {
		return "", skerr.Fmt("Failed to find dependency with id %q", id)
	}

	// Replace the old version with the new.
	lines := strings.Split(content, "\n")
	updateDep(lines, dep.versionPos, dep.Version, version)
	updateDep(lines, dep.sha256Pos, dep.SHA256, sha256)
	return strings.Join(lines, "\n"), nil
}

// updateDep updates a dependency version in the given file content lines.
func updateDep(contentLines []string, pos *ast.Pos, old, new string) {
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
		//     digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
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
				dep, foundDep, err := parseCall(exprStmt.(*ast.Call))
				if err != nil {
					return nil, skerr.Wrap(err)
				}
				if foundDep {
					if err := dep.Validate(); err != nil {
						return nil, skerr.Wrap(err)
					}
					if _, ok := deps[dep.ID]; ok {
						return nil, skerr.Fmt("found multiple entries for %q", dep.ID)
					}
					deps[dep.ID] = dep
				}
			}
		}
	}
	return deps, nil
}

// parseCall parses a Call to return a dependency and the position where its
// version is defined, if a dependency can be found. The boolean return value
// indicates whether a dependency was found.
func parseCall(call *ast.Call) (Dependency, bool, error) {
	var funcName string
	fn, ok := call.Func.(*ast.Name)
	if ok {
		funcName = string(fn.Id)
	} else {
		attr, ok := call.Func.(*ast.Attribute)
		if !ok {
			return Dependency{}, false, skerr.Fmt("Unknown type for function call at line %d", call.Lineno)
		}
		obj, ok := attr.Value.(*ast.Name)
		if !ok {
			return Dependency{}, false, skerr.Fmt("Unknown type for attribute value on function call at line %d", call.Lineno)
		}
		funcName = string(obj.Id) + "." + string(attr.Attr)
	}
	parseFn := map[string]func(*ast.Call) (Dependency, error){
		"cipd_install":   parseDepFromCallFunc("cipd_package", "tag"),
		"container_pull": parseContainerPull,
		"oci.pull":       parseOCIPull,
	}[funcName]
	if parseFn != nil {
		dep, err := parseFn(call)
		if err != nil {
			return Dependency{}, false, skerr.Wrapf(err, "parsing %q call at line %d", funcName, call.Lineno)
		}
		return dep, true, nil
	}
	return Dependency{}, false, nil
}

// parseDepFromCall is a helper function used to extract a dependency from a
// Call using the given idKeyword and versionKeyword.
func parseDepFromCallFunc(idKeyword, versionKeyword string) func(*ast.Call) (Dependency, error) {
	return func(call *ast.Call) (Dependency, error) {
		id, _, err := getCallArgValueString(call, idKeyword)
		if err != nil {
			return Dependency{}, skerr.Wrap(err)
		}
		version, versionPos, err := getCallArgValueString(call, versionKeyword)
		if err != nil {
			return Dependency{}, skerr.Wrap(err)
		}
		sha256, sha256Pos, err := getCallArgValueString(call, "sha256")
		if err != nil {
			return Dependency{}, skerr.Wrap(err)
		}
		return Dependency{
			ID:         DependencyID(id),
			Version:    version,
			versionPos: versionPos,
			SHA256:     sha256,
			sha256Pos:  sha256Pos,
		}, nil
	}
}

// parseContainerPull parses a call to container_pull to return a dependency and
// the position where the version is defined.
func parseContainerPull(call *ast.Call) (Dependency, error) {
	registry, _, err := getCallArgValueString(call, "registry")
	if err != nil {
		return Dependency{}, skerr.Wrap(err)
	}
	repository, _, err := getCallArgValueString(call, "repository")
	if err != nil {
		return Dependency{}, skerr.Wrap(err)
	}
	id := registry + "/" + repository
	digest, digestPos, err := getCallArgValueString(call, "digest")
	if err != nil {
		return Dependency{}, skerr.Wrap(err)
	}
	return Dependency{
		ID:         DependencyID(id),
		Version:    digest,
		versionPos: digestPos,
		SHA256:     digest,
		sha256Pos:  digestPos,
	}, nil
}

// parseOCIPull parses a call to oci_pull to return a dependency and
// the position where the version is defined.
func parseOCIPull(call *ast.Call) (Dependency, error) {
	repository, _, err := getCallArgValueString(call, "repository")
	if err != nil {
		return Dependency{}, skerr.Wrap(err)
	}
	digest, digestPos, err := getCallArgValueString(call, "digest")
	if err != nil {
		return Dependency{}, skerr.Wrap(err)
	}
	return Dependency{
		ID:         DependencyID(repository),
		Version:    digest,
		versionPos: digestPos,
		SHA256:     digest,
		sha256Pos:  digestPos,
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
