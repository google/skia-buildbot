package pyl

/*
Package pyl provides utilities for reading and writing .pyl files.  These
contain a single Python literal expression which is typically made up of nested
dicts and/or lists.  We define a path format which determines how we traverse
the literal in order to find where a particular revision is specified.

For example, the path "key1.key2.id=my-dependency-id.revision" would
traverse the following literal to find the revision ID "12345":

{
  "key1": {
    "key2": [
      {
        "id": "my-dependency-id",
        "revision": "12345",
      },
    ],
  },
}

Path elements indicate either dictionary keys or selectors which match a
property of a list element.
*/

import (
	"reflect"
	"strings"

	"github.com/go-python/gpython/ast"
	"github.com/go-python/gpython/parser"
	"go.skia.org/infra/go/skerr"
)

// pathElem describes one element in a path into a .pyl file.
type pathElem struct {
	Key        string
	ValueMatch string
}

// parsePath parses a path to identify a particular element in the form of,
// "key1.key2.key3=value3.key4"
func parsePath(path string) ([]*pathElem, error) {
	pathSplit := strings.Split(path, ".")
	rv := make([]*pathElem, 0, len(pathSplit))
	for _, elem := range pathSplit {
		equalSplit := strings.Split(elem, "=")
		pathElem := &pathElem{
			Key: equalSplit[0],
		}
		if len(equalSplit) > 1 {
			pathElem.ValueMatch = equalSplit[1]
		}
		rv = append(rv, pathElem)
	}
	return rv, nil
}

// Get retrieves the value at the given path.
func Get(contents, path string) (string, error) {
	// Parse the path.
	pathElems, err := parsePath(path)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	// Parse the .pyl file as a Python expression.
	parsed, err := parser.ParseString(contents, "eval")
	if err != nil {
		return "", skerr.Wrap(err)
	}
	expr, ok := parsed.(*ast.Expression)
	if !ok {
		return "", skerr.Fmt("Parsed value is not an expression: %s", reflect.TypeOf(parsed))
	}
	elem, _, err := getElem(expr.Body, pathElems)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to parse expression")
	}
	return elem, nil
}

// Set sets the value at the given path.
func Set(contents, path, value string) (string, error) {
	// Parse the path.
	pathElems, err := parsePath(path)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	// Parse the .pyl file as a Python expression.
	parsed, err := parser.ParseString(contents, "eval")
	if err != nil {
		return "", skerr.Wrap(err)
	}
	expr, ok := parsed.(*ast.Expression)
	if !ok {
		return "", skerr.Fmt("Parsed value is not an expression: %s", reflect.TypeOf(parsed))
	}
	elem, pos, err := getElem(expr.Body, pathElems)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to parse expression")
	}
	contentsLines := strings.Split(contents, "\n")
	lineIdx := pos.Lineno - 1 // Lineno starts at 1.
	line := contentsLines[lineIdx]
	newLine := line[:pos.ColOffset] + strings.Replace(line[pos.ColOffset:], elem, value, 1)
	contentsLines[lineIdx] = newLine
	return strings.Join(contentsLines, "\n"), nil
}

// getElem retrieves the string at the given path as well as its position within
// the source file.
func getElem(expr ast.Expr, path []*pathElem) (string, ast.Pos, error) {
	if len(path) == 0 {
		if str, ok := expr.(*ast.Str); ok {
			return string(str.S), str.Pos, nil
		}
		return "", ast.Pos{}, skerr.Fmt("expected string at end of path but found %s", expr.Type())
	}
	pathEle := path[0]
	if dict, ok := expr.(*ast.Dict); ok {
		// Note: we're just assuming the use of string keys for dictionaries.
		matchIndex := -1
		for idx, keyExpr := range dict.Keys {
			keyStr, ok := keyExpr.(*ast.Str)
			if !ok {
				return "", ast.Pos{}, skerr.Fmt("expected string key for dictionary but found %q", keyExpr.Type())
			}
			key := string(keyStr.S)
			if key == pathEle.Key {
				matchIndex = idx
			}
		}
		if matchIndex == -1 {
			return "", ast.Pos{}, skerr.Fmt("no matching entry for key %q", pathEle.Key)
		}
		valExpr := dict.Values[matchIndex]
		return getElem(valExpr, path[1:])
	} else if list, ok := expr.(*ast.List); ok {
		for _, elemExpr := range list.Elts {
			dict, ok := elemExpr.(*ast.Dict)
			if !ok {
				return "", ast.Pos{}, skerr.Fmt("expected list of dicts but found %v", elemExpr)
			}
			matchIndex := -1
			for idx, keyExpr := range dict.Keys {
				keyStr, ok := keyExpr.(*ast.Str)
				if !ok {
					return "", ast.Pos{}, skerr.Fmt("expected string key for dictionary but found %q", keyExpr.Type())
				}
				key := string(keyStr.S)
				if key == pathEle.Key {
					matchIndex = idx
				}
			}
			if matchIndex == -1 {
				return "", ast.Pos{}, skerr.Fmt("no matching entry for key %q", pathEle.Key)
			}
			valExpr := dict.Values[matchIndex]
			if strExp, ok := valExpr.(*ast.Str); ok && string(strExp.S) == pathEle.ValueMatch {
				return getElem(elemExpr, path[1:])
			}
		}
		return "", ast.Pos{}, skerr.Fmt("no matching entry with key %s == %s in %+v", pathEle.Key, pathEle.ValueMatch, list)
	} else {
		return "", ast.Pos{}, skerr.Fmt("expected list or dictionary before end of path but found %+v", expr)
	}
}
