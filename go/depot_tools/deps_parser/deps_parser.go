package deps_parser

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/go-python/gpython/ast"
	_ "github.com/go-python/gpython/builtin"
	"github.com/go-python/gpython/compile"
	"github.com/go-python/gpython/parser"
	"github.com/go-python/gpython/py"
	pysys "github.com/go-python/gpython/sys"
	"github.com/go-python/gpython/vm"
	"go.skia.org/infra/go/skerr"
)

const (
	// depsPrefix is prepended to the DEPS file before executing.
	depsPrefix = `
def Var(v):
  return vars[v]
`
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

// versionPos indicates the position in the DEPS file where a DepsEntry's
// version is defined.
type versionPos struct {
	start ast.Pos
	end   ast.Pos
}

// ParseDeps parses the DEPS file content and returns a slice of DepsEntry. It
// does not attempt to understand the full Python syntax upon which DEPS is
// based and may break completely if the file takes an unexpected format.
func ParseDeps(depsContent string) ([]*DepsEntry, error) {
	entries, _, err := parseDeps(depsContent)
	return entries, skerr.Wrap(err)
}

func ParseDepsCompile(r io.Reader) ([]*DepsEntry, error) {
	// Prepare the DEPS file content.
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	depsContent := depsPrefix + string(b)

	// Use gpython to execute the DEPS file. This is mostly copied from
	// https://github.com/go-python/gpython/blob/f4ab05fd08c73c8c6aa967b8c4b3ca3bff1cdd86/main.go
	sys, err := py.GetModule("sys")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	sys.Globals["argv"] = pysys.MakeArgv([]string{"DEPS"})
	obj, err := compile.Compile(depsContent, "DEPS", "exec", 0, false)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	code := obj.(*py.Code)
	module := py.NewModule("__main__", "", nil, nil)
	module.Globals["__file__"] = py.String("DEPS")
	if _, err := vm.Run(module.Globals, module.Globals, code, nil); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Retrieve the deps dict from the globals.
	deps := module.Globals["deps"]
	if deps.Type().Name != py.StringDictType.Name {
		return nil, skerr.Fmt("deps variable is of wrong type; expected %q but got %q", py.StringDictType.Name, deps.Type().Name)
	}
	depsDict := deps.(py.StringDict)

	// Iterate through the deps dict, creating a DepsEntry for each entry.
	rv := []*DepsEntry{}
	for path, entry := range depsDict {
		// DEPS entries may be either strings or dicts; plain strings
		// are git dependencies in '<repo>@<revision>' format. Dicts
		// represent either a git dependency with a conditional or one
		// or more CIPD dependencies.
		t := entry.Type().Name
		var url string
		var dict py.StringDict
		if t == py.StringType.Name {
			url = string(entry.(py.String))
		} else if t == py.StringDictType.Name {
			dict = entry.(py.StringDict)
			if urlVal, ok := dict["url"]; ok {
				if urlVal.Type().Name != py.StringType.Name {
					return nil, skerr.Fmt("wrong type for \"url\" in %q; expected %q but got %q", path, py.StringType.Name, urlVal.Type().Name)
				}
				url = string(urlVal.(py.String))
			}
		} else {
			return nil, skerr.Fmt("unknown deps entry type %q for %q", t, path)
		}
		if url != "" {
			// This is a git dependency. Split the URL into repo and
			// revision, and add the DepsEntry.
			split := strings.SplitN(url, "@", 2)
			if len(split) != 2 {
				return nil, skerr.Fmt("wrong format for string entry; expected \"<repo>@<revision>\" but got %q", url)
			}
			rv = append(rv, &DepsEntry{
				Id:      split[0],
				Version: split[1],
				Path:    path,
			})
		} else if pkgs, ok := dict["packages"]; ok {
			// This is a CIPD dependency(ies).
			// NOTE: The dict should also include a
			// `'dep_type': 'cipd'` entry, but it seems unnecessary
			// to check that, given that the presence of "packages"
			// (and lack of any other type of entry) indicates that
			// this is a CIPD dep.
			if pkgs.Type().Name != py.ListType.Name {
				return nil, skerr.Fmt("wrong type for \"packages\" in %q; expected %q but got %q", path, py.ListType.Name, pkgs.Type().Name)
			}
			pkgsList := pkgs.(*py.List)
			for _, pkg := range pkgsList.Items {
				if pkg.Type().Name != py.StringDictType.Name {
					return nil, skerr.Fmt("wrong type for CIPD package in %q; expected %q but got %q", path, py.StringDictType.Name, pkg.Type().Name)
				}
				pkgDict := pkg.(py.StringDict)
				depsEntry := &DepsEntry{
					Path: path,
				}
				// We should have both "package" and "version"
				// keys.
				if name, ok := pkgDict["package"]; ok && name.Type().Name == py.StringType.Name {
					depsEntry.Id = string(name.(py.String))
				} else {
					return nil, skerr.Fmt("missing entry or wrong type for \"package\" in %q: %+v", path, pkgDict)
				}
				if ver, ok := pkgDict["version"]; ok && ver.Type().Name == py.StringType.Name {
					depsEntry.Version = string(ver.(py.String))
				} else {
					return nil, skerr.Fmt("missing entry or wrong type for \"version\" in %q: %+v", path, pkgDict)
				}
				rv = append(rv, depsEntry)
			}
		} else {
			return nil, skerr.Fmt("don't understand DEPS entry %q which has neither a 'url' or 'packages' key: %+v", path, dict)
		}
	}
	return rv, nil
}

// literalToString resolves the given expression to a string. The returned
// ast.Pos attempts to reflect the location of the version definition, if any,
// within the Expr.
func literalToString(expr ast.Expr) (string, ast.Pos, error) {
	t := expr.Type().Name
	if t == ast.StrType.Name {
		str := expr.(*ast.Str)
		return string(str.S), str.Pos, nil
	} else if t == ast.NameConstantType.Name && expr.(*ast.NameConstant).Value.Type().Name == py.BoolType.Name {
		b := expr.(*ast.NameConstant)
		return fmt.Sprintf("%v", bool(b.Value.(py.Bool))), b.Pos, nil
	} else if t == ast.BinOpType.Name {
		binOp := expr.(*ast.BinOp)
		// We only support addition of strings.
		if binOp.Op != ast.Add {
			return "", ast.Pos{}, skerr.Fmt("Unsupported binop type %q", binOp.Op)
		}
		left, _, err := literalToString(binOp.Left)
		if err != nil {
			return "", ast.Pos{}, skerr.Wrap(err)
		}
		right, pos, err := literalToString(binOp.Right)
		if err != nil {
			return "", ast.Pos{}, skerr.Wrap(err)
		}
		return left + right, pos, nil
	} else {
		return "", ast.Pos{}, skerr.Fmt("Invalid value type %q", t)
	}
}

// resolveDepsEntries resolves an ast.Expr to []*DepsEntry and []as.Pos.
func resolveDepsEntries(vars map[string]ast.Expr, path string, expr ast.Expr) ([]*DepsEntry, []ast.Pos, error) {
	// First, resolve calls to Var().
	expr, err := resolveVars(vars, expr)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// Entries may take one of a few formats.
	if expr.Type().Name == ast.DictType.Name {
		dict := expr.(*ast.Dict)
		// This entry has more detail, either a
		// CIPD package list or a git dep with a
		// conditional.
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
				var poss []ast.Pos
				for _, pkgExpr := range val.(*ast.List).Elts {
					if pkgExpr.Type().Name != ast.DictType.Name {
						return nil, nil, skerr.Fmt("Invalid type for CIPD package list entry at %q; expected %q but got %q", path, ast.DictType.Name, pkgExpr.Type().Name)
					}
					pkgDict := pkgExpr.(*ast.Dict)
					entry := &DepsEntry{
						Path: path,
					}
					var pos ast.Pos
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
							pos = val.(*ast.Str).Pos
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
	t := expr.Type().Name
	if t == ast.StrType.Name || t == ast.BinOpType.Name {
		// String; format is "<repo>@<revision>"
		str, pos, err := literalToString(expr)
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
		}, []ast.Pos{pos}, nil
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
		call := expr.(*ast.Call)
		if len(call.Args) != 1 {
			return nil, skerr.Fmt("Calls to Var() must have a single argument")
		}
		// TODO(borenet): This only handles basic literals!
		key, _, err := literalToString(call.Args[0])
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

// parseDeps parses the DEPS file content and returns a slice of DepsEntry and a
// slice of versionPos which match up to the DepsEntry, or any error which
// occurred.
func parseDeps(depsContent string) ([]*DepsEntry, []ast.Pos, error) {
	// Use gpython to parse the DEPS file as a Python script.
	parsed, err := parser.ParseString(depsContent, "exec")
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// Loop through the statements in the DEPS file.
	rvEntries := []*DepsEntry{}
	rvPos := []ast.Pos{}
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
					rvEntries = append(rvEntries, entries...)
					rvPos = append(rvPos, pos...)
				}
			}
		}
	}
	return rvEntries, rvPos, nil
}
