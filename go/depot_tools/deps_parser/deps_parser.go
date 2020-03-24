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
	"fmt"
	"strings"

	"github.com/go-python/gpython/ast"
	_ "github.com/go-python/gpython/builtin"
	"github.com/go-python/gpython/parser"
	"github.com/go-python/gpython/py"
	"go.skia.org/infra/go/skerr"
)

// DepsEntry represents a single entry in a DEPS file. Note that the 'deps' dict
// may specify that multiple CIPD package are unpacked to the same location; a
// DepsEntry refers to the dependency, not the path, so each CIPD package would
// get its own DepsEntry despite their sharing one key in the 'deps' dict.
type DepsEntry struct {
	// Id describes what the DepsEntry points to, eg. for Git dependencies
	// it is the repo URL, and for CIPD packages it is the package name.
	Id string

	// Version is the currently-pinned version of this dependency.
	Version string

	// Path is the path to which the dependency should be downloaded. It is
	// also used as the key in the 'deps' map in the DEPS file.
	Path string
}

// ParseDeps parses the DEPS file content and returns a map of dependency ID to
// DepsEntry. It does not attempt to understand the full Python syntax upon
// which DEPS is based and may break completely if the file takes an unexpected
// format.
func ParseDeps(depsContent string) (map[string]*DepsEntry, error) {
	entries, _, err := parseDeps(depsContent)
	return entries, err
}

// SetDep parses the DEPS file content, replaces the given dependency with the
// given new version, and returns the new DEPS file content. It does not attempt
// to understand the full Python syntax upon which DEPS is based and may break
// completely if the file takes an unexpected format.
func SetDep(depsContent, depId, version string) (string, error) {
	// Parse the DEPS content.
	entries, poss, err := parseDeps(depsContent)
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
	} else {
		return "", nil, skerr.Fmt("Invalid value type %q", t)
	}
}

// resolveDepsEntries resolves an ast.Expr to []*DepsEntry and []*ast.Pos.
func resolveDepsEntries(vars map[string]ast.Expr, path string, expr ast.Expr) ([]*DepsEntry, []*ast.Pos, error) {
	// First, resolve calls to Var().
	expr, err := resolveVars(vars, expr)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// Entries may take one of a few formats.
	if expr.Type().Name == ast.DictType.Name {
		dict := expr.(*ast.Dict)
		// This entry has more detail, either a CIPD package list or a
		// git dep with a conditional.
		found := false
		for idx, key := range dict.Keys {
			if key.Type().Name != ast.StrType.Name {
				return nil, nil, skerr.Fmt("Invalid type for deps entry dict key %q for %q", key.Type().Name, path)
			}
			strKey := key.(*ast.Str).S
			val := dict.Values[idx]
			if strKey == "url" {
				// This is a git dependency; we'll decode the
				// URL below.
				expr = val
				found = true
			} else if strKey == "packages" {
				// This is a CIPD entry, which represents at
				// least one dependency.
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
					}
					var pos *ast.Pos
					for idx, key := range pkgDict.Keys {
						if key.Type().Name != ast.StrType.Name {
							return nil, nil, skerr.Fmt("Invalid type for CIPD package dict key at %q; expected %q but got %q", path, ast.StrType.Name, key.Type().Name)
						}
						strKey := key.(*ast.Str).S
						val := pkgDict.Values[idx]
						if val.Type().Name != ast.StrType.Name {
							return nil, nil, skerr.Fmt("Invalid type for CIPD package dict value at %q; expected %q but got %q", path, ast.StrType.Name, val.Type().Name)
						}
						strVal := string(val.(*ast.Str).S)
						if strKey == "package" {
							entry.Id = strVal
						} else if strKey == "version" {
							entry.Version = strVal
							pos = &val.(*ast.Str).Pos
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
		if !found {
			return nil, nil, skerr.Fmt("Unable to find dependency in deps entry dict for %q", path)
		}
	}
	// This is a git dependency.
	t := expr.Type().Name
	if t == ast.StrType.Name || t == ast.BinOpType.Name {
		// This could be either a single string, in "<repo>@<revision>"
		// format, or some composition of multiple strings and calls to
		// Var(). Use exprToString() to resolve to a single string.
		str, pos, err := exprToString(expr)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		split := strings.SplitN(str, "@", 2)
		if len(split) != 2 {
			return nil, nil, skerr.Fmt("Invalid dep format; expected <repo>@<revision> but got: %s", str)
		}
		return []*DepsEntry{
			{
				Id:      split[0],
				Version: split[1],
				Path:    path,
			},
		}, []*ast.Pos{pos}, nil
	}
	return nil, nil, skerr.Fmt("Invalid value type %q", t)
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
	}
	// This is a non-recursive or unsupported type. Return expr unchanged.
	return expr, nil
}

// parseDeps parses the DEPS file content and returns a map of dependency ID to
// DepsEntry and a map of dependency to ast.Pos indicating where the dependency
// version was defined in the DEPS file content.
func parseDeps(depsContent string) (map[string]*DepsEntry, map[string]*ast.Pos, error) {
	// Use gpython to parse the DEPS file as a Python script.
	parsed, err := parser.ParseString(depsContent, "exec")
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// Loop through the statements in the DEPS file.
	rvEntries := map[string]*DepsEntry{}
	rvPos := map[string]*ast.Pos{}
	vars := map[string]ast.Expr{}
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
			keys := make([]string, 0, len(d.Keys))
			for _, key := range d.Keys {
				// Only support string keys; in theory they
				// could be any expression, but in practice
				// they're strings.
				if key.Type().Name == ast.StrType.Name {
					keys = append(keys, string(key.(*ast.Str).S))
				} else {
					return nil, nil, skerr.Fmt("Invalid key type for %q: %s", name.Id, key.Type().Name)
				}
			}
			if name.Id == "vars" {
				// Store all vars to be used later.
				for idx, val := range d.Values {
					key := keys[idx]
					vars[key] = val
				}
			} else if name.Id == "deps" {
				// Resolve the deps entries using the vars dict.
				for idx, val := range d.Values {
					entries, pos, err := resolveDepsEntries(vars, keys[idx], val)
					if err != nil {
						return nil, nil, skerr.Wrap(err)
					}
					for idx, entry := range entries {
						rvEntries[entry.Id] = entry
						rvPos[entry.Id] = pos[idx]
					}
				}
			}
		}
	}
	return rvEntries, rvPos, nil
}
