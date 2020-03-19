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
	Id      string
	Version string
	Path    string
}

// ParseDeps parses the DEPS file content and returns a slice of DepsEntry. It
// does not attempt to understand the full Python syntax upon which DEPS is
// based and may break completely if the file takes an unexpected format.
func ParseDeps(r io.Reader) ([]*DepsEntry, error) {
	// Prepare the DEPS file content.
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	depsContent := depsPrefix + string(b)

	// Use gpython to execute the DEPS file. This is mostly copied from
	// https://github.com/go-python/gpython/blob/f4ab05fd08c73c8c6aa967b8c4b3ca3bff1cdd86/main.go
	py.MustGetModule("sys").Globals["argv"] = pysys.MakeArgv([]string{"DEPS"})
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

// TODO: fix
// ParseDeps parses the DEPS file content and returns a slice of DepsEntry. It
// does not attempt to understand the full Python syntax upon which DEPS is
// based and may break completely if the file takes an unexpected format.
func ParseDepsExperiment(r io.Reader) (rv []*DepsEntry, rvErr error) {
	// Use gpython to parse the DEPS file as a Python script.
	parsed, err := parser.Parse(r, "DEPS", "exec")
	if err != nil {
		return nil, err
	}
	// Assume any panics below are due to failed type assertions; this makes
	// the code significantly more readable.
	defer func() {
		if r := recover(); r != nil {
			rvErr = skerr.Fmt("Failed parsing: %v", r)
		}
	}()

	// Loop through the statements in the DEPS file.
	vars := map[string]string{}
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
				return nil, skerr.Fmt("Found different numbers of keys and values for %q", name.Id)
			}
			keys := make([]string, 0, len(d.Keys))
			for _, key := range d.Keys {
				// Only support string keys; in theory they
				// could be any expression, but in practice
				// they're strings.
				if key.Type().Name == ast.StrType.Name {
					keys = append(keys, string(key.(*ast.Str).S))
				} else {
					return nil, skerr.Fmt("Invalid key type for %q: %s", name.Id, key.Type().Name)
				}
			}
			if name.Id == "vars" {
				// Coerce all vars to strings. We only support
				// strings and bools.
				vals := make([]string, 0, len(d.Values))
				for _, val := range d.Values {
					if val.Type().Name == ast.StrType.Name {
						vals = append(vals, string(val.(*ast.Str).S))
					} else if val.Type().Name == ast.NameConstantType.Name &&
						val.(*ast.NameConstant).Value.Type().Name == py.BoolType.Name {
						vals = append(vals, fmt.Sprintf("%v", bool(val.(*ast.NameConstant).Value.(py.Bool))))
					} else {
						return nil, fmt.Errorf("Invalid value type for %q: %s", name.Id, val.Type().Name)
					}
				}
				for idx, key := range keys {
					vars[key] = vals[idx]
				}
			} else if name.Id == "deps" {
				var resolveVarsInExpr func(ast.Expr) ast.Expr
				resolveVarsInExpr = func(expr ast.Expr) ast.Expr {
					if expr.Type().Name == ast.BinOpType.Name {
						binOp := expr.(*ast.BinOp)
						return &ast.BinOp{
							ExprBase: binOp.ExprBase,
							Left:     resolveVarsInExpr(binOp.Left),
							Op:       binOp.Op,
							Right:    resolveVarsInExpr(binOp.Right),
						}
					}
					return expr
				}
				for _, val := range d.Values {
					// Entries may take one of a few formats.
					if val.Type().Name == ast.StrType.Name {
						// String; format is "<repo>@<revision>"
					} else if val.Type().Name == ast.DictType.Name {
						// This entry has more detail, either a
						// CIPD package list or a git dep with a
						// conditional.
					} else if val.Type().Name == ast.BinOpType.Name {
						//
					} else {
						return nil, fmt.Errorf("Invalid value type for %q: %s", name.Id, val.Type().Name)
					}
				}
			}
		}
	}
	return
}
