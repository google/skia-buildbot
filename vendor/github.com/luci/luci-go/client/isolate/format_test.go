// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolate

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/isolated"
	"github.com/maruel/ut"
)

func TestReadOnlyValue(t *testing.T) {
	t.Parallel()
	ut.AssertEqual(t, (*isolated.ReadOnlyValue)(nil), NotSet.ToIsolated())
	ut.AssertEqual(t, (*isolated.ReadOnlyValue)(nil), ReadOnlyValue(100).ToIsolated())
	tmp := new(isolated.ReadOnlyValue)
	*tmp = isolated.Writeable
	ut.AssertEqual(t, tmp, Writeable.ToIsolated())
	*tmp = isolated.FilesReadOnly
	ut.AssertEqual(t, tmp, FilesReadOnly.ToIsolated())
	*tmp = isolated.DirsReadOnly
	ut.AssertEqual(t, tmp, DirsReadOnly.ToIsolated())
}

func TestConditionJson(t *testing.T) {
	t.Parallel()
	c := condition{Condition: "OS == \"Linux\""}
	c.Variables.ReadOnly = new(int)
	c.Variables.Command = []string{"python", "generate", "something"}
	c.Variables.Files = []string{"generated"}
	jsonData, err := json.Marshal(&c)
	ut.AssertEqual(t, nil, err)
	t.Logf("Generated jsonData: %s", string(jsonData))
	pc := condition{}
	err = json.Unmarshal(jsonData, &pc)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, c, pc)
}

func TestVariableValueOrder(t *testing.T) {
	t.Parallel()
	I := func(i int) variableValue { return variableValue{nil, &i} }
	S := func(s string) variableValue { return variableValue{&s, nil} }
	unbound := variableValue{}
	expectations := []struct {
		res int
		l   variableValue
		r   variableValue
	}{
		{0, unbound, unbound},
		{1, unbound, I(1)},
		{1, unbound, S("s")},
		{0, I(1), I(1)},
		{1, I(1), I(2)},
		{1, I(1), S("1")},
		{0, S("s"), S("s")},
		{1, S("a"), S("b")},
	}
	for _, e := range expectations {
		ut.AssertEqualf(t, e.res, e.l.compare(e.r), "%d != (%v < %v)", e.res, e.l.String(), e.r.String())
		ut.AssertEqualf(t, -e.res, e.r.compare(e.l), "%d != (%v > %v)", -e.res, e.l.String(), e.r.String())
	}
}

func TestConfigNameComparison(t *testing.T) {
	t.Parallel()
	C := func(s ...string) configName { return configName(makeVVs(s...)) }
	ut.AssertEqual(t, true, C().Equals(C()))
	ut.AssertEqual(t, 1, C("unbound").compare(C("1")))
}

func TestProcessConditionBad(t *testing.T) {
	t.Parallel()
	expectations := []string{
		"wrong condition1",
		"invalidConditionOp is False",
		"a == 1.1", // Python isolate_format is actually OK with this.
		"a = 1",
	}
	for i, e := range expectations {
		_, err := processCondition(condition{Condition: e}, variablesValuesSet{})
		ut.AssertEqualIndex(t, i, true, err != nil)
	}
}

func TestConditionEvaluate(t *testing.T) {
	t.Parallel()
	const (
		T int = 1
		F int = 0
		E int = -1
	)
	expectations := []struct {
		cond string
		vals map[string]string
		exp  int
	}{
		{"A=='w'", map[string]string{"A": "w"}, T},
		{"A=='m'", map[string]string{"A": "w"}, F},
		{"A=='m'", map[string]string{"a": "w"}, E},
		{"A==1 or B=='b'", map[string]string{"A": "1"}, T},
		{"A==1 or B=='b'", map[string]string{"B": "b"}, E},
		{"A==1 and B=='b'", map[string]string{"A": "1"}, E},

		{"(A=='w')", map[string]string{"A": "w"}, T},
		{"A==1 or (B==2 and C==3)", map[string]string{"A": "1"}, T},
		{"A==1 or (B==2 and C==3)", map[string]string{"A": "0"}, E},
		{"A==1 or (B==2 and C==3)", map[string]string{"A": "0", "B": "0"}, F},
		{"A==1 or (B==2 and C==3)", map[string]string{"A": "2", "B": "2"}, E},
		{"A==1 or (B==2 and C==3)", map[string]string{"A": "2", "B": "2", "C": "3"}, T},

		{"(A==1 or A==2) and B==3", map[string]string{"A": "1", "B": "3"}, T},
		{"(A==1 or A==2) and B==3", map[string]string{"A": "2", "B": "3"}, T},
		{"(A==1 or A==2) and B==3", map[string]string{"B": "3"}, E},
	}
	for i, e := range expectations {
		c, err := processCondition(condition{Condition: e.cond}, variablesValuesSet{})
		ut.AssertEqualIndex(t, i, nil, err)
		isTrue, err := c.evaluate(func(v string) variableValue {
			if value, ok := e.vals[v]; ok {
				return makeVariableValue(value)
			}
			assert(variableValue{}.isBound() == false)
			return variableValue{}
		})
		if e.exp == E {
			ut.AssertEqualIndex(t, i, errors.New("required variable is unbound"), err)
		} else {
			ut.AssertEqualIndex(t, i, nil, err)
			ut.AssertEqualIndex(t, i, e.exp == T, isTrue)
		}
	}
}

func TestMatchConfigs(t *testing.T) {
	t.Parallel()
	unbound := "unbound" // Treated specially by makeVVs to create unbound variableValue.
	expectations := []struct {
		cond string
		conf []string
		all  [][]variableValue
		out  [][]variableValue
	}{
		{"OS==\"win\"", []string{"OS"},
			[][]variableValue{makeVVs("win"), makeVVs("mac"), makeVVs("linux")},
			[][]variableValue{makeVVs("win")},
		},
		{"(foo==1 or foo==2) and bar==\"b\"", []string{"foo", "bar"},
			[][]variableValue{makeVVs("1", "a"), makeVVs("1", "b"), makeVVs("2", "a"), makeVVs("2", "b")},
			[][]variableValue{makeVVs("1", "b"), makeVVs("2", "b")},
		},
		{"bar==\"b\"", []string{"foo", "bar"},
			[][]variableValue{makeVVs("1", "a"), makeVVs("1", "b"), makeVVs("2", "a"), makeVVs("2", "b")},
			[][]variableValue{makeVVs("1", "b"), makeVVs("2", "b"), makeVVs(unbound, "b")},
		},
		{"foo==1 or bar==\"b\"", []string{"foo", "bar"},
			[][]variableValue{makeVVs("1", "a"), makeVVs("1", "b"), makeVVs("2", "a"), makeVVs("2", "b")},
			[][]variableValue{makeVVs("1", "a"), makeVVs("1", "b"), makeVVs("2", "b"), makeVVs("1", unbound)},
		},
	}
	for i, e := range expectations {
		c, err := processCondition(condition{Condition: e.cond}, variablesValuesSet{})
		ut.AssertEqualIndex(t, i, nil, err)
		out := c.matchConfigs(makeConfigVariableIndex(e.conf), e.all)
		ut.AssertEqualIndex(t, i, vvToStr2D(vvSort(e.out)), vvToStr2D(vvSort(out)))
	}
}

func TestCartesianProductOfValues(t *testing.T) {
	t.Parallel()
	set := func(vs ...string) map[variableValueKey]variableValue {
		out := map[variableValueKey]variableValue{}
		for _, v := range makeVVs(vs...) {
			out[v.key()] = v
		}
		return out
	}
	test := func(vvs variablesValuesSet, keys []string, expected ...[]variableValue) {
		res, err := vvs.cartesianProductOfValues(keys)
		ut.AssertEqual(t, nil, err)
		vvSort(expected)
		vvSort(res)
		ut.AssertEqual(t, vvToStr2D(expected), vvToStr2D(res))
	}
	keys := func(vs ...string) []string { return vs }

	vvs := variablesValuesSet{}
	test(vvs, keys())

	vvs["OS"] = set("win", "unbound")
	test(vvs, keys("OS"), makeVVs("win"), makeVVs("unbound"))

	vvs["bit"] = set("32")

	test(vvs, keys("OS"), makeVVs("win"), makeVVs("unbound")) // bit var name must be ignored.
	test(vvs, keys("bit", "OS"), makeVVs("32", "win"), makeVVs("32", "unbound"))
}

func TestParseBadIsolate(t *testing.T) {
	t.Parallel()
	// These tests make Python spill out errors into stderr, which might be confusing.
	// However, when real isolate command runs, we want to see these errors.
	badIsolates := []string{
		"statement = 'is not good'",
		"{'includes': map(str, [1,2])}",
		"{ python/isolate syntax error",
		"{'wrong_section': False, 'includes': 'must be a list'}",
		"{'conditions': ['must be list of conditions', {}]}",
		"{'conditions': [['', {'variables-missing': {}}]]}",
		"{'variables': ['bad variables type']}",
	}
	for i, badIsolate := range badIsolates {
		_, err := processIsolate([]byte(badIsolate))
		ut.AssertEqualIndex(t, i, true, err != nil)
	}
}

func TestPythonToGoString(t *testing.T) {
	t.Parallel()
	expectations := []struct {
		in, out, left string
	}{
		{`''`, `""`, ``},
		{`''\'`, `""`, `\'`},
		{`'''`, `""`, `'`},
		{`""`, `""`, ``},
		{`"" and`, `""`, ` and`},
		{`'"' "w`, `"\""`, ` "w`},
		{`"'"`, `"'"`, ``},
		{`"\'"`, `"'"`, ``},
		{`"ok" or`, `"ok"`, ` or`},
		{`'\\"\\'`, `"\\\"\\"`, ``},
		{`"\\\"\\"`, `"\\\"\\"`, ``},
		{`"'\\'"`, `"'\\'"`, ``},
		{`'"\'\'"'`, `"\"''\""`, ``},
		{`'"\'\'"'`, `"\"''\""`, ``},
		{`'∀ unicode'`, `"∀ unicode"`, ``},
	}
	for i, e := range expectations {
		goChunk, left, err := pythonToGoString(stringToRunes(e.in))
		t.Logf("in: `%s` eg: `%s` g: `%s` el: `%s` l: `%s` err: %s", e.in, e.out, goChunk, e.left, string(left), err)
		ut.AssertEqualIndex(t, i, e.left, string(left))
		ut.AssertEqualIndex(t, i, e.out, goChunk)
		ut.AssertEqualIndex(t, i, nil, err)
	}
}

func TestPythonToGoStringError(t *testing.T) {
	t.Parallel()
	expErr := errors.New("failed to parse Condition string")
	for i, e := range []string{`'"`, `"'`, `'\'`, `"\"`, `'""`, `"''`} {
		goChunk, left, err := pythonToGoString(stringToRunes(e))
		t.Logf("in: `%s`, g: `%s`, l: `%s`, err: %s", e, goChunk, string(left), err)
		ut.AssertEqualIndex(t, i, expErr, err)
	}
}

func TestPythonToGoNonString(t *testing.T) {
	t.Parallel()
	expectations := []struct {
		in, out, left string
	}{
		{`and`, `&&`, ``},
		{`or`, `||`, ``},
		{` or('str'`, ` ||(`, `'str'`},
		{`)or(`, `)||(`, ``},
		{`andor`, `andor`, ``},
		{`)whatever("string...`, `)whatever(`, `"string...`},
	}
	for i, e := range expectations {
		goChunk, left := pythonToGoNonString(stringToRunes(e.in))
		t.Logf("in: `%s` eg: `%s` g: `%s` el: `%s` l: `%s`", e.in, e.out, goChunk, e.left, string(left))
		ut.AssertEqualIndex(t, i, e.left, string(left))
		ut.AssertEqualIndex(t, i, e.out, goChunk)
	}
}

func TestVerifyVariables(t *testing.T) {
	t.Parallel()
	v := variables{}
	badRo := -2
	v.ReadOnly = &badRo
	ut.AssertEqual(t, true, v.verify() != nil)
}

const sampleIncludes = `
	'includes': [
		'inc/included.isolate',
	],`

const sampleIsolateData = `
# This is file comment to be ignored.
{
	'conditions': [
		['(OS=="linux" and bit==64) or OS=="win"', {
			'variables': {
				'command': ['python', '64linuxOrWin'],
				'files': [
					'64linuxOrWin',
					'<(PRODUCT_DIR)/unittest<(EXECUTABLE_SUFFIX)',
				],
			},
		}],
		['bit==32 or (OS=="mac" and bit==64)', {
			'variables': {
				'command': ['python', '32orMac64'],
				'read_only': 2,
			},
		}],
	],
}`

const sampleIncIsolateData = `
{
	'conditions': [
		['OS=="linux" or OS=="win" or OS=="mac"', {
			'variables': {
				'command': ['to', 'be', 'ignored'],
				'files': [
					'inc_file',
					'<(DIR)/inc_unittest',
				],
			},
		}],
	],
}`

var sampleIsolateDataWithIncludes = addIncludesToSample(sampleIsolateData, sampleIncludes)

func TestProcessIsolate(t *testing.T) {
	t.Parallel()
	p, err := processIsolate([]byte(sampleIsolateDataWithIncludes))
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, `(OS=="linux" and bit==64) or OS=="win"`, p.conditions[0].condition)
	ut.AssertEqual(t, 2, len(p.conditions[0].variables.Files))
	ut.AssertEqual(t, []string{"inc/included.isolate"}, p.includes)

	vars, ok := getSortedVarValues(p.varsValsSet, "OS")
	ut.AssertEqual(t, true, ok)
	ut.AssertEqual(t, vvToStr(makeVVs("linux", "mac", "win")), vvToStr(vars))
	vars, ok = getSortedVarValues(p.varsValsSet, "bit")
	ut.AssertEqual(t, true, ok)
	ut.AssertEqual(t, vvToStr(makeVVs("32", "64")), vvToStr(vars))
}

func TestLoadIsolateAsConfig(t *testing.T) {
	t.Parallel()
	root := "/dir"
	if common.IsWindows() {
		root = "x:\\dir"
	}
	isolate, err := LoadIsolateAsConfig(root, []byte(sampleIsolateData))
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, []string{"OS", "bit"}, isolate.ConfigVariables)
}

func TestLoadIsolateForConfigMissingVars(t *testing.T) {
	t.Parallel()
	isoData := []byte(sampleIsolateData)
	root := "/dir"
	if common.IsWindows() {
		root = "x:\\dir"
	}
	_, _, _, _, err := LoadIsolateForConfig(root, isoData, nil)
	ut.AssertEqual(t, true, err != nil)
	ut.AssertEqualf(t, true, strings.Contains(err.Error(), "variables were missing"), "%s", err)
	ut.AssertEqualf(t, true, strings.Contains(err.Error(), "bit"), "%s", err)
	ut.AssertEqualf(t, true, strings.Contains(err.Error(), "OS"), "%s", err)
	_, _, _, _, err = LoadIsolateForConfig(root, isoData, map[string]string{"bit": "32"})
	ut.AssertEqual(t, true, err != nil)
	ut.AssertEqualf(t, true, strings.Contains(err.Error(), "variables were missing"), "%s", err)
	ut.AssertEqualf(t, true, strings.Contains(err.Error(), "OS"), "%s", err)
}

func TestLoadIsolateForConfig(t *testing.T) {
	t.Parallel()
	// Case linux64, matches first condition.
	root := "/dir"
	if common.IsWindows() {
		root = "x:\\dir"
	}
	vars := map[string]string{"bit": "64", "OS": "linux"}
	cmd, deps, ro, dir, err := LoadIsolateForConfig(root, []byte(sampleIsolateData), vars)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, root, dir)
	ut.AssertEqual(t, NotSet, ro) // first condition has no read_only specified.
	ut.AssertEqual(t, []string{"python", "64linuxOrWin"}, cmd)
	ut.AssertEqual(t, []string{"64linuxOrWin", filepath.Join("<(PRODUCT_DIR)", "unittest<(EXECUTABLE_SUFFIX)")}, deps)

	// Case win64, matches only first condition.
	vars = map[string]string{"bit": "64", "OS": "win"}
	cmd, deps, ro, dir, err = LoadIsolateForConfig(root, []byte(sampleIsolateData), vars)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, root, dir)
	ut.AssertEqual(t, NotSet, ro) // first condition has no read_only specified.
	ut.AssertEqual(t, []string{"python", "64linuxOrWin"}, cmd)
	ut.AssertEqual(t, []string{
		"64linuxOrWin",
		filepath.Join("<(PRODUCT_DIR)", "unittest<(EXECUTABLE_SUFFIX)"),
	}, deps)

	// Case mac64, matches only second condition.
	vars = map[string]string{"bit": "64", "OS": "mac"}
	cmd, deps, ro, dir, err = LoadIsolateForConfig(root, []byte(sampleIsolateData), vars)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, root, dir)
	ut.AssertEqual(t, DirsReadOnly, ro) // second condition has read_only 2.
	ut.AssertEqual(t, []string{"python", "32orMac64"}, cmd)
	ut.AssertEqual(t, []string{}, deps)

	// Case win32, both first and second condition match.
	vars = map[string]string{"bit": "32", "OS": "win"}
	cmd, deps, ro, dir, err = LoadIsolateForConfig(root, []byte(sampleIsolateData), vars)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, root, dir)
	ut.AssertEqual(t, DirsReadOnly, ro) // first condition no read_only, but second has 2.
	ut.AssertEqual(t, []string{"python", "32orMac64"}, cmd)
	ut.AssertEqual(t, []string{
		"64linuxOrWin",
		filepath.Join("<(PRODUCT_DIR)", "unittest<(EXECUTABLE_SUFFIX)"),
	}, deps)
}

func TestLoadIsolateAsConfigWithIncludes(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "test-isofmt-")
	ut.AssertEqual(t, nil, err)
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fail()
		}
	}()
	err = os.Mkdir(filepath.Join(tmpDir, "inc"), 0777)
	ut.AssertEqual(t, nil, err)
	err = ioutil.WriteFile(filepath.Join(tmpDir, "inc", "included.isolate"), []byte(sampleIncIsolateData), 0777)
	ut.AssertEqual(t, nil, err)

	// Test failures.
	absIncData := addIncludesToSample(sampleIsolateData, "'includes':['/abs/path']")
	_, _, _, _, err = LoadIsolateForConfig(tmpDir, []byte(absIncData), nil)
	ut.AssertEqual(t, true, err != nil)

	_, _, _, _, err = LoadIsolateForConfig(filepath.Join(tmpDir, "wrong-dir"),
		[]byte(sampleIsolateDataWithIncludes), nil)
	ut.AssertEqual(t, true, err != nil)

	// Test Successfull loading.
	// Case mac32, matches only second condition from main isolate and one in included.
	vars := map[string]string{"bit": "64", "OS": "linux"}
	cmd, deps, ro, dir, err := LoadIsolateForConfig(tmpDir, []byte(sampleIsolateDataWithIncludes), vars)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, tmpDir, dir)
	ut.AssertEqual(t, NotSet, ro) // first condition has no read_only specified.
	ut.AssertEqual(t, []string{"python", "64linuxOrWin"}, cmd)
	ut.AssertEqual(t, []string{
		"64linuxOrWin",
		filepath.Join("<(DIR)", "inc_unittest"), // no rebasing for this.
		filepath.Join("<(PRODUCT_DIR)", "unittest<(EXECUTABLE_SUFFIX)"),
		filepath.Join("inc", "inc_file"),
	}, deps)
}

func TestConfigSettingsUnionLeft(t *testing.T) {
	t.Parallel()
	left := &ConfigSettings{
		Command:    []string{"left takes precedence"},
		Files:      []string{"../../le/f/t", "foo/"}, // Must be POSIX.
		IsolateDir: absToOS("/tmp/bar"),              // In native path.
	}
	right := &ConfigSettings{
		Files:      []string{"../ri/g/ht", "bar/"},
		IsolateDir: absToOS("/var/lib"),
	}

	out, err := left.union(right)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, left.Command, out.Command)
	ut.AssertEqual(t, left.IsolateDir, out.IsolateDir)
	ut.AssertEqual(t, absToOS("/tmp/bar"), left.IsolateDir)
	ut.AssertEqual(t, []string{"../../le/f/t", "../../var/lib/bar/", "../../var/ri/g/ht", "foo/"}, out.Files)
}

func TestConfigSettingsUnionRight(t *testing.T) {
	t.Parallel()
	left := &ConfigSettings{
		Files:      []string{"../../le/f/t", "foo/"}, // Must be POSIX.
		IsolateDir: absToOS("/tmp/bar"),              // In native path.
	}
	right := &ConfigSettings{
		Command:    []string{"right takes precedence"},
		Files:      []string{"../ri/g/ht", "bar/"},
		IsolateDir: absToOS("/var/lib"),
	}

	out, err := left.union(right)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, right.Command, out.Command)
	ut.AssertEqual(t, right.IsolateDir, out.IsolateDir)
	ut.AssertEqual(t, absToOS("/var/lib"), right.IsolateDir)
	ut.AssertEqual(t, []string{"../../le/f/t", "../../tmp/bar/foo/", "../ri/g/ht", "bar/"}, out.Files)
}

// Helper functions.

// absToOS converts a POSIX path to OS specific format.
func absToOS(p string) string {
	if common.IsWindows() {
		return "e:" + strings.Replace(p, "/", "\\", -1)
	}
	return p
}

// makeVVs simplifies creating variableValue:
// "unbound" => unbound
// "123" => int(123)
// "s123" => string("123")
func makeVVs(ss ...string) []variableValue {
	vs := make([]variableValue, len(ss))
	for i, s := range ss {
		if s == "unbound" {
			continue
		} else if strings.HasPrefix(s, "s") {
			vs[i] = makeVariableValue(s[1:])
			if vs[i].I == nil {
				vs[i].S = &s
			}
		} else {
			vs[i] = makeVariableValue(s)
		}
	}
	return vs
}

func vvToStr(vs []variableValue) []string {
	ks := make([]string, len(vs))
	for i, v := range vs {
		ks[i] = v.String()
	}
	return ks
}

func vvToStr2D(vs [][]variableValue) [][]string {
	ks := make([][]string, len(vs))
	for i, v := range vs {
		ks[i] = vvToStr(v)
	}
	return ks
}

func vvSort(vss [][]variableValue) [][]variableValue {
	tmpMap := map[string][]variableValue{}
	keys := []string{}
	for _, vs := range vss {
		key := configName(vs).key()
		keys = append(keys, key)
		tmpMap[key] = vs
	}
	sort.Strings(keys)
	for i, key := range keys {
		vss[i] = tmpMap[key]
	}
	return vss
}

func addIncludesToSample(sample, includes string) string {
	assert(sample[len(sample)-1] == '}')
	return sample[:len(sample)-1] + includes + "\n}"
}

func getSortedVarValues(v variablesValuesSet, varName string) ([]variableValue, bool) {
	valueSet, ok := v[varName]
	if !ok {
		return nil, false
	}
	keys := make([]string, 0, len(valueSet))
	for key := range valueSet {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)
	values := make([]variableValue, len(valueSet))
	for i, key := range keys {
		values[i] = valueSet[variableValueKey(key)]
	}
	return values, true
}
