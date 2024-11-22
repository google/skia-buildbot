package deps_parser

/*
Package deps_parser provides tools for parsing and updating DEPS files.

Doing this outside of depot tools proper is arguably a bad idea; we're doing it
for the following reasons:

1. Using depot tools requires adding a Python installation to the Docker image,
   which accounts for ~200MB of space.
2. There's going to be some churn as a result of switching to Python 3. It makes
   more sense to just stop using Python than to jump through all of the hoops in
   order to update.
3. gclient doesn't actually have a command to dump all dependency versions in a
   machine-readable format. "gclient revinfo" comes close, but it returns the
   actually-synced versions, which isn't quite what we want and requires a
   checkout. Writing this code was just as easy as adding the desired
   functionality to gclient.
4. Between the fakery (eg. writing a .gclient file) required to make gclient
   happy in the absence of an actual checkout, the environment variables needed
   to prevent depot tools from updating itself away from our pinned version,
   the extra time needed to shell out to Python, etc, having to use gclient is
   just generally a pain.

Note that we could have taken the easier route and write a tiny Python script
to dump the DEPS, but we'd still need a solution for updating entries (simple
find-and-replace won't work because different CIPD packages may use the same
version tags). Points 1 and 2 from above would still apply in that scenario as
well.
*/

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-python/gpython/ast"
	_ "github.com/go-python/gpython/builtin"
	"github.com/go-python/gpython/parser"
	"github.com/go-python/gpython/py"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

const (
	// DepsFileName is the name of the DEPS file.
	DepsFileName = "DEPS"
)

var (
	// We treat "{var_name}" in strings equivalently to a call to Var().
	varSubstRegex = regexp.MustCompile(`{?{(\w+?)}}?`)

	// These are built-in variables provided by gclient and usable in conditions
	// and Vars() in DEPS files.
	// Note: gclient also provides a number of "checkout_*" variables based on
	// the platform; we don't attempt to provide those here since they are
	// typically only used in conditionals, which are ignored by this package.
	builtinVars = map[string]string{
		"host_os":  "linux",
		"host_cpu": "x64",
	}
)

type DepType string

const (
	DepType_Git  DepType = "git"
	DepType_Cipd DepType = "cipd"
	DepType_Gcs  DepType = "gcs"
)

// DepsEntry represents a single entry in a DEPS file. Note that the 'deps' dict
// may specify that multiple CIPD package are unpacked to the same location; a
// DepsEntry refers to the dependency, not the path, so each CIPD package would
// get its own DepsEntry despite their sharing one key in the 'deps' dict.
type DepsEntry struct {
	// Id describes what the DepsEntry points to, eg. for Git dependencies
	// it is the normalized repo URL, and for CIPD packages it is the
	// package name.
	Id string

	// Version is the currently-pinned version of this dependency.
	Version string

	// Path is the path to which the dependency should be downloaded. It is
	// also used as the key in the 'deps' map in the DEPS file.
	Path string

	// Type indicates the type of the dependency.
	Type DepType
}

// DepsEntries represents all entries in a DEPS file.
type DepsEntries map[string]*DepsEntry

// Get retrieves the DepsEntry with the given ID, accounting for normalization.
// Returns the DepsEntry or nil if the entry was not found.
func (e DepsEntries) Get(dep string) *DepsEntry {
	return e[NormalizeDep(dep)]
}

// ParseDeps parses the DEPS file content and returns a map of normalized
// dependency ID to DepsEntry. It does not attempt to understand the full Python
// syntax upon which DEPS is based and may break completely if the file takes an
// unexpected format.
func ParseDeps(depsContent string) (DepsEntries, error) {
	entries, _, err := parseDeps(depsContent, true)
	return entries, err
}

// ParseDepsNoNormalize is like ParseDeps but does not normalize dependency IDs.
func ParseDepsNoNormalize(depsContent string) (DepsEntries, error) {
	entries, _, err := parseDeps(depsContent, false)
	return entries, err
}

// GetDep parses the given depsContent and retrieves the given DepsEntry.
// Returns an error if the dep was not found.
func GetDep(depsContent, dep string) (*DepsEntry, error) {
	entries, err := ParseDeps(depsContent)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	entry := entries.Get(dep)
	if entry == nil {
		b, err := json.MarshalIndent(entries, "", "  ")
		if err == nil {
			return nil, skerr.Fmt("Unable to find %q in %s! Entries:\n%s", dep, DepsFileName, string(b))
		} else {
			return nil, skerr.Fmt("Unable to find %q in %s! Failed to encode DEPS entries with: %s", dep, DepsFileName, err)
		}
	}
	return entry, nil
}

// SetDep parses the DEPS file content, replaces the given dependency with the
// given new version, and returns the new DEPS file content. It does not attempt
// to understand the full Python syntax upon which DEPS is based and may break
// completely if the file takes an unexpected format.
func SetDep(depsContent, depId, version string) (string, error) {
	// Normalize the dependency ID.
	depId = NormalizeDep(depId)

	// Parse the DEPS content.
	entries, poss, err := parseDeps(depsContent, true)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Find the requested dependency.
	dep := entries[depId]
	pos := poss[depId]
	if dep == nil || pos == nil {
		return "", skerr.Fmt("Failed to find dependency with id %q", depId)
	}

	// Replace the old version with the new.
	depsLines := strings.Split(depsContent, "\n")
	lineIdx := pos.Lineno - 1 // Lineno starts at 1.
	line := depsLines[lineIdx]
	newLine := line[:pos.ColOffset] + strings.Replace(line[pos.ColOffset:], dep.Version, version, 1)
	depsLines[lineIdx] = newLine
	return strings.Join(depsLines, "\n"), nil
}

// HasGitSubmodules
func HasGitSubmodules(depsContent string) (bool, error) {
	parsed, err := parser.ParseString(depsContent, "exec")
	if err != nil {
		return false, skerr.Wrap(err)
	}
	for _, stmt := range parsed.(*ast.Module).Body {
		// We only care about assignment statements.
		if stmt.Type().Name != ast.AssignType.Name {
			continue
		}
		assign := stmt.(*ast.Assign)
		for _, target := range assign.Targets {
			// We only care about assignments to global variables,
			// by name.
			if target.Type().Name != ast.NameType.Name {
				continue
			}
			name := target.(*ast.Name)
			if name.Id == "git_dependencies" {
				v := string(assign.Value.(*ast.Str).S)
				return v == "SYNC" || v == "SUBMODULES", nil
			}
		}
	}
	return false, nil
}

// exprToString resolves the given expression to a string. The returned
// ast.Pos attempts to reflect the location of the version definition, if any,
// within the Expr.
func exprToString(expr ast.Expr) (string, *ast.Pos, error) {
	t := expr.Type().Name
	if t == ast.StrType.Name {
		str := expr.(*ast.Str)
		return string(str.S), &str.Pos, nil
	} else if t == ast.NameConstantType.Name && expr.(*ast.NameConstant).Value.Type().Name == py.BoolType.Name {
		b := expr.(*ast.NameConstant)
		return fmt.Sprintf("%v", bool(b.Value.(py.Bool))), &b.Pos, nil
	} else if t == ast.BinOpType.Name {
		binOp := expr.(*ast.BinOp)
		// We only support addition of strings.
		if binOp.Op != ast.Add {
			return "", nil, skerr.Fmt("Unsupported binop type %q", binOp.Op)
		}
		left, _, err := exprToString(binOp.Left)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		right, pos, err := exprToString(binOp.Right)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		// Just assume that the version is always the part of the
		// expression furthest to the right.
		return left + right, pos, nil
	} else if t == ast.NumType.Name {
		num := expr.(*ast.Num)
		return fmt.Sprintf("%d", num.N), &num.Pos, nil
	} else {
		return "", nil, skerr.Fmt("Invalid value type %q", t)
	}
}

// resolveDepsEntries resolves an ast.Expr to []*DepsEntry and []*ast.Pos.
func resolveDepsEntries(vars map[string]ast.Expr, key, value ast.Expr) ([]*DepsEntry, []*ast.Pos, error) {
	// First, resolve calls to Var().
	keyExpr, err := resolveVars(vars, key)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	valueExpr, err := resolveVars(vars, value)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// Resolve the key to a string, the path to the dep.
	path, _, err := exprToString(keyExpr)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// If the entry is a dictionary, it may be a Git, CIPD, or GCS dependency.
	if valueExpr.Type().Name == ast.DictType.Name {
		// Find the type for the entry.
		dict := valueExpr.(*ast.Dict)
		for idx, key := range dict.Keys {
			if key.Type().Name != ast.StrType.Name {
				return nil, nil, skerr.Fmt("Invalid type for deps entry dict key %q for %q", key.Type().Name, path)
			}
			strKey := key.(*ast.Str).S
			val := dict.Values[idx]
			if strKey == "dep_type" {
				if val.Type().Name != ast.StrType.Name {
					return nil, nil, skerr.Fmt("invalid type for key `dep_type` %q", key.Type().Name)
				}
				depType := string(val.(*ast.Str).S)
				if depType == "git" {
					return parseGitDep(path, dict)
				} else if depType == "cipd" {
					return parseCIPDDeps(path, dict)
				} else if depType == "gcs" {
					return parseGCSDeps(path, dict)
				}
			} else if strKey == "url" {
				// `url` indicates that this is a git dep, and `dep_type` may
				// not be set.
				return parseGitDep(path, dict)
			} else if strKey == "bucket" {
				// `bucket` indicates that this is a GCS dep, and `dep_type` may
				// not be set.
				return parseGCSDeps(path, dict)
			}
		}
		return nil, nil, skerr.Fmt("unable to find dependency type in deps entry dict for %q", path)
	}

	// Otherwise, we assume it's a Git dependency.
	return parseGitDep(path, valueExpr)
}

func parseGitDep(path string, valueExpr ast.Expr) ([]*DepsEntry, []*ast.Pos, error) {
	if valueExpr.Type().Name == ast.DictType.Name {
		// If this is a dictionary, just grab the "url" key.
		dict := valueExpr.(*ast.Dict)
		for idx, key := range dict.Keys {
			strKey := key.(*ast.Str).S
			val := dict.Values[idx]
			if strKey == "url" {
				valueExpr = val
			}
		}
	}

	t := valueExpr.Type().Name
	if t == ast.StrType.Name || t == ast.BinOpType.Name {
		// This could be either a single string, in "<repo>@<revision>"
		// format, or some composition of multiple strings and calls to
		// Var(). Use exprToString() to resolve to a single string.
		str, pos, err := exprToString(valueExpr)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		split := strings.SplitN(str, "@", 2)
		entry := &DepsEntry{
			Id:   split[0],
			Path: path,
			Type: DepType_Git,
		}
		// Some DEPS files contain unpinned entries with no "@version"
		// suffix. This isn't really valid, but we shouldn't fail to
		// parse them. Note that we will not be able to correctly update
		// the version of the dependency via SetDep.
		if len(split) == 2 {
			entry.Version = split[1]
		}
		return []*DepsEntry{entry}, []*ast.Pos{pos}, nil
	} else {
		return nil, nil, skerr.Fmt("invalid type %q for Git dependency at %q", t, path)
	}
}

func parseCIPDDeps(path string, dict *ast.Dict) ([]*DepsEntry, []*ast.Pos, error) {
	// This is a CIPD entry, which represents at least one package.
	for idx, key := range dict.Keys {
		strKey := key.(*ast.Str).S
		val := dict.Values[idx]
		if strKey == "packages" {
			if val.Type().Name != ast.ListType.Name {
				return nil, nil, skerr.Fmt("Invalid type for %q at %q; expected %q but got %q", strKey, path, ast.ListType.Name, val.Type().Name)
			}
			var entries []*DepsEntry
			var poss []*ast.Pos
			for _, pkgExpr := range val.(*ast.List).Elts {
				if pkgExpr.Type().Name != ast.DictType.Name {
					return nil, nil, skerr.Fmt("Invalid type for CIPD package list entry at %q; expected %q but got %q", path, ast.DictType.Name, pkgExpr.Type().Name)
				}
				pkgDict := pkgExpr.(*ast.Dict)
				entry := &DepsEntry{
					Path: path,
					Type: DepType_Cipd,
				}
				var pos *ast.Pos
				for idx, key := range pkgDict.Keys {
					if key.Type().Name != ast.StrType.Name {
						return nil, nil, skerr.Fmt("Invalid type for CIPD package dict key at %q; expected %q but got %q", path, ast.StrType.Name, key.Type().Name)
					}
					strKey := key.(*ast.Str).S
					var strVal string
					var err error
					strVal, exprPos, err := exprToString(pkgDict.Values[idx])
					if err != nil {
						return nil, nil, skerr.Wrap(err)
					}
					if strKey == "package" {
						entry.Id = strVal
					} else if strKey == "version" {
						entry.Version = strVal
						pos = exprPos
					}
				}
				if entry.Id == "" || entry.Version == "" {
					return nil, nil, skerr.Fmt("CIPD package dict for %q is incomplete", path)
				}
				entries = append(entries, entry)
				poss = append(poss, pos)
			}
			return entries, poss, nil
		}
	}
	return nil, nil, skerr.Fmt("missing `packages` key for CIPD dependency %q", path)
}

func parseGCSDeps(path string, dict *ast.Dict) ([]*DepsEntry, []*ast.Pos, error) {
	// This is a GCS dependency, which represents at least one object.
	var bucket string
	var objects []string
	var sha256sums []string
	var poss []*ast.Pos

	for idx, key := range dict.Keys {
		strKey := key.(*ast.Str).S
		val := dict.Values[idx]
		var err error
		if strKey == "bucket" {
			bucket, _, err = exprToString(val)
			if err != nil {
				return nil, nil, skerr.Wrapf(err, "failed to parse bucket as string")
			}
		} else if strKey == "objects" {
			if val.Type().Name != ast.ListType.Name {
				return nil, nil, skerr.Fmt("Invalid type for %q at %q; expected %q but got %q", strKey, path, ast.ListType.Name, val.Type().Name)
			}
			for _, objectExpr := range val.(*ast.List).Elts {
				if objectExpr.Type().Name != ast.DictType.Name {
					return nil, nil, skerr.Fmt("Invalid type for GCS object list entry at %q; expected %q but got %q", path, ast.DictType.Name, objectExpr.Type().Name)
				}
				objectDict := objectExpr.(*ast.Dict)
				var object string
				var sha256sum string
				var pos *ast.Pos
				for idx, key := range objectDict.Keys {
					if key.Type().Name != ast.StrType.Name {
						return nil, nil, skerr.Fmt("Invalid type for GCS object dict key at %q; expected %q but got %q", path, ast.StrType.Name, key.Type().Name)
					}
					strKey := key.(*ast.Str).S
					var strVal string
					var err error
					strVal, exprPos, err := exprToString(objectDict.Values[idx])
					if err != nil {
						return nil, nil, skerr.Wrap(err)
					}
					if strKey == "object_name" {
						object = strVal
					} else if strKey == "sha256sum" {
						sha256sum = strVal
						pos = exprPos
					}
				}
				if object == "" || sha256sum == "" {
					return nil, nil, skerr.Fmt("GCS object dict for %q is incomplete", path)
				}
				objects = append(objects, object)
				sha256sums = append(sha256sums, sha256sum)
				poss = append(poss, pos)
			}
		}
	}
	if bucket == "" {
		return nil, nil, skerr.Fmt("missing key `bucket` for gcs entry %q", path)
	}
	entries := make([]*DepsEntry, 0, len(objects))
	for idx, object := range objects {
		entries = append(entries, &DepsEntry{
			Id:      fmt.Sprintf("%s/%s", bucket, object),
			Version: sha256sums[idx],
			Path:    path,
			Type:    DepType_Gcs,
		})
	}

	return entries, poss, nil
}

// resolveVars recursively replaces calls to Var() with the ast.Expr for the
// variable itself in the given ast.Expr.
func resolveVars(vars map[string]ast.Expr, expr ast.Expr) (ast.Expr, error) {
	t := expr.Type().Name
	if t == ast.BinOpType.Name {
		binOp := expr.(*ast.BinOp)
		left, err := resolveVars(vars, binOp.Left)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		right, err := resolveVars(vars, binOp.Right)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return &ast.BinOp{
			ExprBase: binOp.ExprBase,
			Left:     left,
			Op:       binOp.Op,
			Right:    right,
		}, nil
	} else if t == ast.DictType.Name {
		dict := expr.(*ast.Dict)
		keys := make([]ast.Expr, 0, len(dict.Keys))
		for _, key := range dict.Keys {
			resolved, err := resolveVars(vars, key)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			keys = append(keys, resolved)
		}
		vals := make([]ast.Expr, 0, len(dict.Values))
		for _, val := range dict.Values {
			resolved, err := resolveVars(vars, val)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			vals = append(vals, resolved)
		}
		return &ast.Dict{
			ExprBase: dict.ExprBase,
			Keys:     keys,
			Values:   vals,
		}, nil
	} else if t == ast.ListType.Name {
		list := expr.(*ast.List)
		elts := make([]ast.Expr, 0, len(list.Elts))
		for _, e := range list.Elts {
			resolved, err := resolveVars(vars, e)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			elts = append(elts, resolved)
		}
		return &ast.List{
			ExprBase: list.ExprBase,
			Elts:     elts,
			Ctx:      list.Ctx,
		}, nil
	} else if t == ast.TupleType.Name {
		tuple := expr.(*ast.Tuple)
		elts := make([]ast.Expr, 0, len(tuple.Elts))
		for _, e := range tuple.Elts {
			resolved, err := resolveVars(vars, e)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			elts = append(elts, resolved)
		}
		return &ast.Tuple{
			ExprBase: tuple.ExprBase,
			Elts:     elts,
			Ctx:      tuple.Ctx,
		}, nil
	} else if t == ast.CallType.Name && expr.(*ast.Call).Func.(*ast.Name).Id == "Var" {
		// This is a vars lookup.
		call := expr.(*ast.Call)
		if len(call.Args) != 1 {
			return nil, skerr.Fmt("Calls to Var() must have a single argument")
		}
		key, _, err := exprToString(call.Args[0])
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		val, ok := vars[key]
		if !ok {
			return nil, skerr.Fmt("No such var: %s", key)
		}
		return val, nil
	} else if t == ast.StrType.Name {
		// Strings may contain vars references in "{var}blah" format.
		str := expr.(*ast.Str)
		matches := varSubstRegex.FindAllStringSubmatchIndex(string(str.S), -1)
		if len(matches) == 0 {
			// No vars references; return the string as-is.
			return expr, nil
		}
		// Gclient performs an implicit `str.format(**vars)` on string
		// literals. Approximate that behavior by breaking formatted
		// strings into a series of expressions.
		prevIdx := 0
		var exprs []ast.Expr
		for _, match := range matches {
			if len(match) != 4 {
				return nil, skerr.Fmt("Invalid format for regex match; expected 4 indexes but got: %+v", match)
			}
			// If there were any characters between the previous
			// match and this one, they become a new string literal.
			if prevIdx < match[0] {
				exprs = append(exprs, &ast.Str{
					ExprBase: str.ExprBase,
					S:        str.S[prevIdx:match[0]],
				})
			}
			// Special case: double-bracketed strings just
			// become single-bracketed.
			if str.S[match[0]:match[2]] == "{{" && str.S[match[3]:match[1]] == "}}" {
				exprs = append(exprs, &ast.Str{
					ExprBase: str.ExprBase,
					S:        str.S[match[0]+1 : match[1]-1],
				})
			} else {
				// Insert the expression for the vars entry.
				key := str.S[match[2]:match[3]]
				val, ok := vars[string(key)]
				if !ok {
					return nil, skerr.Fmt("No such var: %s", key)
				}
				exprs = append(exprs, val)
			}
			prevIdx = match[1]
		}
		// If there are any characters after the last match, they become
		// a new string literal.
		if prevIdx < len(str.S) {
			exprs = append(exprs, &ast.Str{
				ExprBase: str.ExprBase,
				S:        str.S[prevIdx:len(str.S)],
			})
		}
		// Glob the exprs together by repeatedly replacing the last two
		// string literals with a BinOp.
		for len(exprs) > 1 {
			left := exprs[len(exprs)-2]
			right := exprs[len(exprs)-1]
			expr := &ast.BinOp{
				ExprBase: str.ExprBase,
				Left:     left,
				Op:       ast.Add,
				Right:    right,
			}
			exprs[len(exprs)-2] = expr
			exprs = exprs[:len(exprs)-1]
		}
		return exprs[0], nil
	}
	// This is a non-recursive or unsupported type. Return expr unchanged.
	return expr, nil
}

// parseDeps parses the DEPS file content and returns a map of normalized
// dependency ID to DepsEntry and a map of normalized dependency ID to ast.Pos
// indicating where the dependency version was defined in the DEPS file content.
func parseDeps(depsContent string, normalizeIDs bool) (DepsEntries, map[string]*ast.Pos, error) {
	// Use gpython to parse the DEPS file as a Python script.
	parsed, err := parser.ParseString(depsContent, "exec")
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// Loop through the statements in the DEPS file.
	rvEntries := map[string]*DepsEntry{}
	rvPos := map[string]*ast.Pos{}
	vars := map[string]ast.Expr{}
	for k, v := range builtinVars {
		vars[k] = &ast.Str{
			S: py.String(v),
		}
	}
	for _, stmt := range parsed.(*ast.Module).Body {
		// We only care about assignment statements.
		if stmt.Type().Name != ast.AssignType.Name {
			continue
		}
		assign := stmt.(*ast.Assign)
		for _, target := range assign.Targets {
			// We only care about assignments to global variables,
			// by name.
			if target.Type().Name != ast.NameType.Name {
				continue
			}
			name := target.(*ast.Name)
			if name.Ctx != ast.Store {
				continue
			}

			// We only care about deps and vars, which are both dicts.
			if assign.Value.Type().Name != ast.DictType.Name {
				continue
			}
			d := assign.Value.(*ast.Dict)
			if len(d.Keys) != len(d.Values) {
				return nil, nil, skerr.Fmt("Found different numbers of keys and values for %q", name.Id)
			}
			keys := make([]ast.Expr, 0, len(d.Keys))
			keys = append(keys, d.Keys...)
			if name.Id == "vars" {
				// Store all vars to be used later.
				for idx, val := range d.Values {
					// Resolve the key to a string value. This will fail if it's
					// anything complex, eg. a function call.
					key := keys[idx]
					keyStr, _, err := exprToString(key)
					if err != nil {
						return nil, nil, skerr.Wrapf(err, "failed to resolve key expression for vars %q", key)
					}
					vars[keyStr] = val
				}
			} else if name.Id == "deps" {
				// Resolve the deps entries using the vars dict.
				for idx, val := range d.Values {
					entries, pos, err := resolveDepsEntries(vars, keys[idx], val)
					if err != nil {
						return nil, nil, skerr.Wrap(err)
					}
					for idx, entry := range entries {
						if normalizeIDs {
							entry.Id = NormalizeDep(entry.Id)
						}
						rvEntries[entry.Id] = entry
						rvPos[entry.Id] = pos[idx]
					}
				}
			}
		}
	}
	return DepsEntries(rvEntries), rvPos, nil
}

// NormalizeDep normalizes the dependency ID to account for differences, eg.
// the URL scheme and .git suffix for git repos and the ${platform} suffix for
// CIPD packages.
func NormalizeDep(depId string) string {
	// TODO(borenet): Will this adversely affect non-git entries?
	if rv, err := git.NormalizeURL(depId); err == nil {
		// NormalizeURL sometimes adds an undesired leading '/' for
		// entries which aren't actually URLs.
		depId = strings.TrimPrefix(rv, "/")
	}

	// Trim the "${platform}" suffix from CIPD entries.
	depId = strings.TrimSuffix(depId, "/"+cipd.PlatformPlaceholder)
	return depId
}
