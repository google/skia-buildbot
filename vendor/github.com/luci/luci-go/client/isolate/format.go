// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go/ast"
	"go/parser"
	"go/token"

	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/isolated"
	"github.com/yosuke-furukawa/json5/encoding/json5"
)

var osPathSeparator = string(os.PathSeparator)

// ReadOnlyValue defines permissions on isolated files.
type ReadOnlyValue int

// Possible kinds of read only values.
const (
	NotSet        ReadOnlyValue = -1
	Writeable     ReadOnlyValue = 0
	FilesReadOnly ReadOnlyValue = 1
	DirsReadOnly  ReadOnlyValue = 2
)

// ToIsolated can be used to convert ReadOnlyValue enum values to corresponding
// isolated.ReadOnlyValue enum values. It returns nil on errors.
func (r ReadOnlyValue) ToIsolated() (out *isolated.ReadOnlyValue) {
	switch r {
	case NotSet:
	case Writeable:
		out = new(isolated.ReadOnlyValue)
		*out = isolated.Writeable
	case FilesReadOnly:
		out = new(isolated.ReadOnlyValue)
		*out = isolated.FilesReadOnly
	case DirsReadOnly:
		out = new(isolated.ReadOnlyValue)
		*out = isolated.DirsReadOnly
	default:
		log.Printf("invalid ReadOnlyValue %d", r)
	}
	return
}

// LoadIsolateAsConfig parses one .isolate file and returns a Configs instance.
//
//  Arguments:
//    isolateDir: only used to load relative includes so it doesn't depend on
//                cwd.
//    value: is the loaded dictionary that was defined in the gyp file.
//
//  The expected format is strict, anything diverting from the format below will
//  result in error:
//  {
//    'includes': [
//      'foo.isolate',
//    ],
//    'conditions': [
//      ['OS=="vms" and foo=42', {
//        'variables': {
//          'command': [
//            ...
//          ],
//          'files': [
//            ...
//          ],
//          'read_only': 0,
//        },
//      }],
//      ...
//    ],
//    'variables': {
//      ...
//    },
//  }
func LoadIsolateAsConfig(isolateDir string, content []byte) (*Configs, error) {
	// isolateDir must be in native style.
	if !filepath.IsAbs(isolateDir) {
		return nil, fmt.Errorf("%s is not an absolute path", isolateDir)
	}
	processedIsolate, err := processIsolate(content)
	if err != nil {
		return nil, fmt.Errorf("failed to process isolate (isolateDir: %s): %s", isolateDir, err)
	}
	out := processedIsolate.toConfigs()
	// Add global variables. The global variables are on the empty tuple key.
	globalconfigName := make([]variableValue, len(out.ConfigVariables))
	out.setConfig(globalconfigName, newConfigSettings(processedIsolate.variables, isolateDir))

	// Add configuration-specific variables.
	allConfigs, err := processedIsolate.getAllConfigs(out.ConfigVariables)
	if err != nil {
		return nil, err
	}
	configVariablesIndex := makeConfigVariableIndex(out.ConfigVariables)
	for _, cond := range processedIsolate.conditions {
		newConfigs := newConfigs(out.ConfigVariables)
		configs := cond.matchConfigs(configVariablesIndex, allConfigs)
		for _, config := range configs {
			newConfigs.setConfig(configName(config), newConfigSettings(cond.variables, isolateDir))
		}
		if out, err = out.union(newConfigs); err != nil {
			return nil, err
		}
	}
	// If the .isolate contains command, ignore any command in child .isolate.
	rootHasCommand := false
	for _, pair := range out.byConfig {
		if len(pair.value.Command) > 0 {
			rootHasCommand = true
			break
		}
	}
	// Load the includes. Process them in reverse so the last one take precedence.
	for i := len(processedIsolate.includes) - 1; i >= 0; i-- {
		included, err := loadIncludedIsolate(isolateDir, processedIsolate.includes[i])
		if err != nil {
			return nil, err
		}
		if rootHasCommand {
			// Strip any command in the imported isolate. It is because the chosen
			// command is not related to the one in the top-most .isolate, since the
			// configuration is flattened.
			for _, pair := range included.byConfig {
				pair.value.Command = []string{}
			}
		}
		if out, err = out.union(included); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// LoadIsolateForConfig loads the .isolate file and returns
// the information unprocessed but filtered for the specific OS.
//
// Returns:
//   command, dependencies, readOnly flag, relDir, error.
//
// relDir and dependencies are fixed to use os.PathSeparator.
func LoadIsolateForConfig(isolateDir string, content []byte, configVariables map[string]string) (
	[]string, []string, ReadOnlyValue, string, error) {
	// Load the .isolate file, process its conditions, retrieve the command and dependencies.
	isolate, err := LoadIsolateAsConfig(isolateDir, content)
	if err != nil {
		return nil, nil, NotSet, "", err
	}
	configName := configName{}
	missingVars := []string{}
	for _, variable := range isolate.ConfigVariables {
		if value, ok := configVariables[variable]; ok {
			configName = append(configName, makeVariableValue(value))
		} else {
			missingVars = append(missingVars, variable)
		}
	}
	if len(missingVars) > 0 {
		sort.Strings(missingVars)
		err = fmt.Errorf("these configuration variables were missing from the command line: %v", missingVars)
		return nil, nil, NotSet, "", err
	}
	// A configuration is to be created with all the combinations of free variables.
	config, err := isolate.GetConfig(configName)
	if err != nil {
		return nil, nil, NotSet, "", err
	}
	dependencies := config.Files
	relDir := config.IsolateDir
	if os.PathSeparator != '/' {
		dependencies = make([]string, len(config.Files))
		for i, f := range config.Files {
			dependencies[i] = strings.Replace(f, "/", osPathSeparator, -1)
		}
		relDir = strings.Replace(relDir, "/", osPathSeparator, -1)
	}
	return config.Command, dependencies, config.ReadOnly, relDir, nil
}

func loadIncludedIsolate(isolateDir, include string) (*Configs, error) {
	if filepath.IsAbs(include) {
		return nil, fmt.Errorf("failed to load configuration; absolute include path %s", include)
	}
	includedIsolate := filepath.Clean(filepath.Join(isolateDir, include))
	if common.IsWindows() && (strings.ToLower(includedIsolate)[0] != strings.ToLower(isolateDir)[0]) {
		return nil, errors.New("can't reference a .isolate file from another drive")
	}
	content, err := ioutil.ReadFile(includedIsolate)
	if err != nil {
		return nil, err
	}
	return LoadIsolateAsConfig(filepath.Dir(includedIsolate), content)
}

// Configs represents a processed .isolate file.
//
// Stores the file in a processed way, split by configuration.
//
// At this point, we don't know all the possibilities. So mount a partial view
// that we have.
//
// This class doesn't hold isolateDir, since it is dependent on the final
// configuration selected. It is implicitly dependent on which .isolate defines
// the 'command' that will take effect.
type Configs struct {
	// ConfigVariables contains names only, sorted by name; the order is same as in byConfig.
	ConfigVariables []string
	// The config key are lists of values of vars in the same order as ConfigSettings.
	byConfig map[string]configPair
}

func newConfigs(configVariables []string) *Configs {
	c := &Configs{configVariables, map[string]configPair{}}
	assert(sort.IsSorted(sort.StringSlice(c.ConfigVariables)))
	return c
}

func (c *Configs) getSortedConfigPairs() configPairs {
	pairs := make([]configPair, 0, len(c.byConfig))
	for _, pair := range c.byConfig {
		pairs = append(pairs, pair)
	}
	out := configPairs(pairs)
	sort.Sort(out)
	return out
}

// GetConfig returns all configs that matches this config as a single ConfigSettings.
//
// Returns nil if none apply.
func (c *Configs) GetConfig(configName configName) (*ConfigSettings, error) {
	// Order byConfig according to configNames ordering function.
	out := &ConfigSettings{}
	for _, pair := range c.getSortedConfigPairs() {
		ok := true
		for i, confKey := range configName {
			if pair.key[i].isBound() && pair.key[i].compare(confKey) != 0 {
				ok = false
				break
			}
		}
		if ok {
			var err error
			if out, err = out.union(pair.value); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// setConfig sets the ConfigSettings for this key.
//
// The key is a tuple of bounded or unbounded variables. The global variable
// is the key where all values are unbounded.
func (c *Configs) setConfig(confName configName, value *ConfigSettings) {
	assert(len(confName) == len(c.ConfigVariables))
	assert(value != nil)
	key := confName.key()
	pair, ok := c.byConfig[key]
	assert(!ok, "setConfig must not override existing keys (%s => %v)", key, pair.value)
	c.byConfig[key] = configPair{confName, value}
}

// union returns a new Configs instance, the union of variables from self and rhs.
//
// It keeps ConfigVariables sorted in the output.
func (c *Configs) union(rhs *Configs) (*Configs, error) {
	// Merge the keys of ConfigVariables for each Configs instances. All the new
	// variables will become unbounded. This requires realigning the keys.
	configVariables := uniqueMergeSortedStrings(
		c.ConfigVariables, rhs.ConfigVariables)
	out := newConfigs(configVariables)
	byConfig := configPairs(append(
		c.expandConfigVariables(configVariables),
		rhs.expandConfigVariables(configVariables)...))
	if len(byConfig) == 0 {
		return out, nil
	}
	// Take union of ConfigSettings with the same configName (key),
	// in order left, right.
	// Thus, preserve the order between left, right while sorting.
	sort.Stable(byConfig)
	last := byConfig[0]
	for _, curr := range byConfig[1:] {
		if last.key.compare(curr.key) == 0 {
			val, err := last.value.union(curr.value)
			if err != nil {
				return out, err
			}
			last.value = val
		} else {
			out.setConfig(last.key, last.value)
			last = curr
		}
	}
	out.setConfig(last.key, last.value)
	return out, nil
}

// expandConfigVariables returns new configPair list for newConfigVars.
func (c *Configs) expandConfigVariables(newConfigVars []string) []configPair {
	// Get mapping from old config vars list to new one.
	mapping := make([]int, len(newConfigVars))
	i := 0
	for n, nk := range newConfigVars {
		if i == len(c.ConfigVariables) || c.ConfigVariables[i] > nk {
			mapping[n] = -1
		} else if c.ConfigVariables[i] == nk {
			mapping[n] = i
			i++
		} else {
			// Must never happen because newConfigVars and c.configVariables are sorted ASC,
			// and newConfigVars contain c.configVariables as a subset.
			panic("unreachable code")
		}
	}
	// Expands configName to match newConfigVars.
	getNewconfigName := func(old configName) configName {
		newConfig := make(configName, len(mapping))
		for k, v := range mapping {
			if v != -1 {
				newConfig[k] = old[v]
			}
		}
		return newConfig
	}
	// Compute new byConfig.
	out := make([]configPair, 0, len(c.byConfig))
	for _, pair := range c.byConfig {
		out = append(out, configPair{getNewconfigName(pair.key), pair.value})
	}
	return out
}

func createReadOnlyValue(readOnly *int) ReadOnlyValue {
	if readOnly == nil {
		return NotSet
	}
	return ReadOnlyValue(*readOnly)
}

// ConfigSettings represents the dependency variables for a single build configuration.
//
// The structure is immutable.
type ConfigSettings struct {
	// Files is the list of dependencies. The items use '/' as a path separator.
	Files []string
	// Command is the actual command to run.
	Command []string
	// ReadOnly describes how to map the files.
	ReadOnly ReadOnlyValue
	// IsolateDir is the path where to start the command from.
	// It uses the OS' native path separator and it must be an absolute path.
	IsolateDir string
}

func newConfigSettings(variables variables, isolateDir string) *ConfigSettings {
	if isolateDir == "" {
		// It must be an empty object if isolateDir is not set.
		assert(variables.isEmpty(), variables)
	} else {
		assert(filepath.IsAbs(isolateDir))
	}
	c := &ConfigSettings{
		make([]string, len(variables.Files)),
		variables.Command,
		createReadOnlyValue(variables.ReadOnly),
		isolateDir,
	}
	copy(c.Files, variables.Files)
	sort.Strings(c.Files)
	return c
}

// union merges two config settings together into a new instance.
//
// A new instance is not created and self or rhs is returned if the other
// object is the empty object.
//
// self has priority over rhs for Command. Use the same IsolateDir as the
// one having a Command.
//
// Dependencies listed in rhs are patch adjusted ONLY if they don't start with
// a path variable, e.g. the characters '<('.
func (lhs *ConfigSettings) union(rhs *ConfigSettings) (*ConfigSettings, error) {
	// When an object has IsolateDir == "", it means it is the empty object.
	if lhs.IsolateDir == "" {
		return rhs, nil
	}
	if rhs.IsolateDir == "" {
		return lhs, nil
	}

	if common.IsWindows() && strings.ToLower(lhs.IsolateDir)[0] != strings.ToLower(rhs.IsolateDir)[0] {
		return nil, errors.New("All .isolate files must be on same drive")
	}

	// Takes the difference between the two isolateDir. Note that while
	// isolateDir is in native path case, all other references are in posix.
	useRHS := false
	var command []string
	if len(lhs.Command) > 0 {
		useRHS = false
		command = lhs.Command
	} else if len(rhs.Command) > 0 {
		useRHS = true
		command = rhs.Command
	} else {
		// If self doesn't define any file, use rhs.
		useRHS = len(lhs.Files) == 0
	}

	readOnly := rhs.ReadOnly
	if lhs.ReadOnly != NotSet {
		readOnly = lhs.ReadOnly
	}

	lRelCwd, rRelCwd := lhs.IsolateDir, rhs.IsolateDir
	lFiles, rFiles := lhs.Files, rhs.Files
	if useRHS {
		// Rebase files in rhs.
		lRelCwd, rRelCwd = rhs.IsolateDir, lhs.IsolateDir
		lFiles, rFiles = rhs.Files, lhs.Files
	}

	rebasePath, err := filepath.Rel(lRelCwd, rRelCwd)
	if err != nil {
		return nil, err
	}
	rebasePath = strings.Replace(rebasePath, osPathSeparator, "/", -1)

	filesSet := map[string]bool{}
	for _, f := range lFiles {
		filesSet[f] = true
	}
	for _, f := range rFiles {
		// Rebase item.
		if !(strings.HasPrefix(f, "<(") || rebasePath == ".") {
			// paths are posix here.
			trailingSlash := strings.HasSuffix(f, "/")
			f = path.Join(rebasePath, f)
			if trailingSlash {
				f += "/"
			}
		}
		filesSet[f] = true
	}
	// Remove duplicates.
	files := make([]string, 0, len(filesSet))
	for f := range filesSet {
		files = append(files, f)
	}
	sort.Strings(files)
	return &ConfigSettings{files, command, readOnly, lRelCwd}, nil
}

// Private details.

// isolate represents contents of the isolate file.
// The main purpose is (de)serialization.
type isolate struct {
	Includes   []string    `json:"includes,omitempty"`
	Conditions []condition `json:"conditions"`
	Variables  variables   `json:"variables,omitempty"`
}

// condition represents conditional part of an isolate file.
type condition struct {
	Condition string
	Variables variables
}

// MarshalJSON implements json.Marshaler interface.
func (p *condition) MarshalJSON() ([]byte, error) {
	d := [2]json.RawMessage{}
	var err error
	if d[0], err = json.Marshal(&p.Condition); err != nil {
		return nil, err
	}
	m := map[string]variables{"variables": p.Variables}
	if d[1], err = json.Marshal(&m); err != nil {
		return nil, err
	}
	return json.Marshal(&d)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (p *condition) UnmarshalJSON(data []byte) error {
	var d []json.RawMessage
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	if len(d) != 2 {
		return errors.New("condition must be a list with two items")
	}
	if err := json.Unmarshal(d[0], &p.Condition); err != nil {
		return err
	}
	m := map[string]variables{}
	if err := json.Unmarshal(d[1], &m); err != nil {
		return err
	}
	var ok bool
	if p.Variables, ok = m["variables"]; !ok {
		return errors.New("variables item is required in condition")
	}
	return nil
}

// variables represents variable as part of condition or top level in an isolate file.
type variables struct {
	Command []string `json:"command"`
	Files   []string `json:"files"`
	// ReadOnly has 1 as default, according to specs.
	// Just as Python-isolate also uses None as default, this code uses nil.
	ReadOnly *int `json:"read_only"`
}

// variableValue holds a single value of a string or an int,
// otherwise it is unbound.
type variableValue struct {
	S *string
	I *int
}

func makeVariableValue(s string) variableValue {
	v := variableValue{}
	if i, err := strconv.Atoi(s); err == nil {
		v.I = &i
	} else {
		v.S = &s
	}
	return v
}

func (v variableValue) String() string {
	if v.S != nil {
		return *v.S
	} else if v.I != nil {
		return fmt.Sprintf("%d", *v.I)
	}
	return ""
}

// compare returns 0 if equal, 1 if lhs < right, else -1.
// Order: unbound < 1 < 2 < "abc" < "cde" .
func (v variableValue) compare(rhs variableValue) int {
	if v.I != nil {
		if rhs.I != nil {
			// Both integers.
			if *v.I < *rhs.I {
				return 1
			} else if *v.I > *rhs.I {
				return -1
			}
			return 0
		} else if rhs.S != nil {
			// int vs string
			return 1
		}
		// int vs Unbound.
		return -1
	} else if v.S != nil {
		if rhs.S != nil {
			// Both strings.
			if *v.S < *rhs.S {
				return 1
			} else if *v.S > *rhs.S {
				return -1
			}
			return 0
		}
		// string vs (int | unbound)
		return -1
	} else if rhs.isBound() {
		// unbound vs (int|string)
		return 1
	}
	// unbound vs unbound
	return 0
}

func (v variableValue) isBound() bool {
	return v.S != nil || v.I != nil
}

// variableValueKey is for indexing by variableValue in a map.
type variableValueKey string

func (v variableValue) key() variableValueKey {
	if v.S != nil {
		return variableValueKey("~" + *v.S)
	}
	if v.I != nil {
		return variableValueKey(string(*v.I))
	}
	return variableValueKey("")
}

// variablesValueSet maps variable name to set of possible values
// found in condition strings.
type variablesValuesSet map[string]map[variableValueKey]variableValue

func (v variablesValuesSet) cartesianProductOfValues(orderedKeys []string) ([][]variableValue, error) {
	if len(orderedKeys) == 0 {
		return [][]variableValue{}, nil
	}
	// Prepare ordered by orderedKeys list of variableValue
	allValues := make([][]variableValue, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		valuesSet := v[key]
		values := make([]variableValue, 0, len(valuesSet))
		for _, value := range valuesSet {
			values = append(values, value)
		}
		allValues = append(allValues, values)
	}
	// Precompute length of output for alloc and for assertion at the end.
	length := 1
	for _, values := range allValues {
		length *= len(values)
	}
	if length <= 0 {
		return nil, errors.New("some variable had empty valuesSet?")
	}
	out := make([][]variableValue, 0, length)
	// indices[i] points to index in allValues[i]; stop once indices[-1] == len(allValues[-1]).
	indices := make([]int, len(orderedKeys))
	for {
		next := make([]variableValue, len(orderedKeys))
		for i, values := range allValues {
			if indices[i] == len(values) {
				if i+1 == len(orderedKeys) {
					if length != len(out) {
						return nil, errors.New("internal error")
					}
					return out, nil
				}
				indices[i] = 0
				indices[i+1]++
			}
			next[i] = values[indices[i]]
		}
		out = append(out, next)
		indices[0]++
	}
	// unreachable
}

// processedIsolate is verified Isolate ready for further processing.
// Immutable once created.
type processedIsolate struct {
	includes    []string
	conditions  []*processedCondition
	variables   variables
	varsValsSet variablesValuesSet
}

// convertIsolateToJSON5 cleans up isolate content to be json5.
func convertIsolateToJSON5(content []byte) io.Reader {
	out := &bytes.Buffer{}
	for _, l := range strings.Split(string(content), "\n") {
		l = strings.TrimSpace(l)
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		l = strings.Replace(l, "\"", "\\\"", -1)
		l = strings.Replace(l, "'", "\"", -1)
		_, _ = io.WriteString(out, l+"\n")
	}
	return out
}

func parseIsolate(content []byte) (*isolate, error) {
	isolate := &isolate{}
	// TODO(tandrii): figure out why decoding directly into isolate
	// doesn't work.
	// if err := json5.NewDecoder(json5src).Decode(isolate); err != nil {
	var data interface{}
	if err := json5.NewDecoder(convertIsolateToJSON5(content)).Decode(&data); err != nil {
		return nil, err
	}
	buf, _ := json.Marshal(&data)
	if err := json.Unmarshal(buf, isolate); err != nil {
		return nil, err
	}
	return isolate, nil
}

// processIsolate loads isolate, then verifies and returns it as
// a processedIsolate for faster further processing.
func processIsolate(content []byte) (*processedIsolate, error) {
	isolate, err := parseIsolate(content)
	if err != nil {
		return nil, err
	}
	if err := isolate.Variables.verify(); err != nil {
		return nil, err
	}
	out := &processedIsolate{
		isolate.Includes,
		make([]*processedCondition, len(isolate.Conditions)),
		isolate.Variables,
		variablesValuesSet{},
	}
	for i, cond := range isolate.Conditions {
		out.conditions[i], err = processCondition(cond, out.varsValsSet)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (p *processedIsolate) toConfigs() *Configs {
	configVariables := make([]string, 0, len(p.varsValsSet))
	for varName := range p.varsValsSet {
		configVariables = append(configVariables, varName)
	}
	sort.Strings(configVariables)
	return newConfigs(configVariables)
}

func (p *processedIsolate) getAllConfigs(configVariables []string) ([][]variableValue, error) {
	return p.varsValsSet.cartesianProductOfValues(configVariables)
}

// processedCondition is a verified Condition ready for evaluation.
type processedCondition struct {
	condition string
	variables variables
	expr      ast.Expr
	// equalityValues are cached values of literals in "id==val" parts of
	// Condition, uniquely indexed by their position.
	equalityValues map[token.Pos]variableValue
}

// processCondition ensures condition is in correct format, and converts it
// to processedCondition for further evaluation.
func processCondition(c condition, varsAndValues variablesValuesSet) (*processedCondition, error) {
	goCond, err := pythonToGoCondition(c.Condition)
	if err != nil {
		return nil, err
	}
	out := &processedCondition{condition: c.Condition, variables: c.Variables}
	if out.expr, err = parser.ParseExpr(goCond); err != nil {
		return nil, err
	}
	if out.equalityValues, err = processConditionAst(out.expr, varsAndValues); err != nil {
		return nil, err
	}
	return out, out.variables.verify()
}

func processConditionAst(expr ast.Expr, varsAndValues variablesValuesSet) (map[token.Pos]variableValue, error) {
	err := error(nil)
	equalityValues := map[token.Pos]variableValue{}
	ast.Inspect(expr, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		if err != nil {
			return false
		}
		switch n := n.(type) {
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				return true
			}
			if n.Op != token.EQL {
				err = fmt.Errorf("unknown binary operator %s\n", n.Op)
				return false
			}
			id, value, tmpErr := verifyIDEqualValue(n)
			if tmpErr != nil {
				err = tmpErr
				return false
			}
			equalityValues[n.Pos()] = value
			if _, exists := varsAndValues[id]; !exists {
				varsAndValues[id] = map[variableValueKey]variableValue{}
			}
			varsAndValues[id][value.key()] = value
			return false
		case *ast.ParenExpr:
			return true
		default:
			err = fmt.Errorf("unknown expression type %T\n", n)
			return false
		}
		// unreachable
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Condition: %s", err)
	}
	return equalityValues, nil
}

func (c *processedCondition) matchConfigs(configVariablesIndex map[string]int, allConfigs [][]variableValue) [][]variableValue {
	// Brute force: try all possible subsets of boundVariables and their values
	// (config) from allConfigs for which condition is True.
	// Runs in O(2^len(configVariables) * len(allConfigs)), but in practice it
	// is fast enough.
	if len(configVariablesIndex) > 60 {
		panic(fmt.Errorf("isolate doesn't scale to %d ConfigVariables", len(configVariablesIndex)))
	}
	boundSubsetsCount := int64(1) << uint(len(configVariablesIndex))
	okConfigs := map[string][]variableValue{}
	// boundBits represents current subset of configVariables which are bound.
	for boundBits := int64(0); boundBits < boundSubsetsCount; boundBits++ {
		for _, config := range allConfigs {
			isTrue, err := c.evaluate(func(varName string) variableValue {
				i := configVariablesIndex[varName]
				if (boundBits & (int64(1) << uint(i))) != 0 {
					return config[i]
				}
				return variableValue{}
			})
			if err == nil && isTrue {
				okConfig := make([]variableValue, len(config))
				for i := 0; i < len(config); i++ {
					if (boundBits & (int64(1) << uint(i))) != 0 {
						okConfig[i] = config[i]
					}
				}
				okConfigs[configName(okConfig).key()] = okConfig
			}
		}
	}
	out := make([][]variableValue, 0, len(okConfigs))
	for _, okConfig := range okConfigs {
		out = append(out, okConfig)
	}
	return out
}

type funcGetVariableValue func(varName string) variableValue

func (c *processedCondition) evaluate(getValue funcGetVariableValue) (bool, error) {
	ce := conditionEvaluator{cond: c, getVarValue: getValue, stop: false}
	isTrue := ce.eval(c.expr)
	if ce.stop {
		return false, errors.New("required variable is unbound")
	}
	return isTrue, nil
}

type conditionEvaluator struct {
	cond        *processedCondition
	getVarValue funcGetVariableValue
	stop        bool
}

func (c *conditionEvaluator) eval(e ast.Expr) bool {
	if c.stop {
		return false
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return c.eval(e.X)
	case *ast.BinaryExpr:
		if e.Op == token.LAND {
			return c.eval(e.X) && c.eval(e.Y)
		} else if e.Op == token.LOR {
			return c.eval(e.X) || c.eval(e.Y)
		}
		assert(e.Op == token.EQL)
		value := c.getVarValue(e.X.(*ast.Ident).Name)
		if !value.isBound() {
			c.stop = true
			return false
		}
		eqValue := c.cond.equalityValues[e.Pos()]
		assert(eqValue.isBound())
		return value.compare(c.cond.equalityValues[e.Pos()]) == 0
	default:
		panic(errors.New("processCondition must have ensured condition is evaluatable"))
	}
	// unreachable
}

func makeConfigVariableIndex(configVariables []string) map[string]int {
	out := map[string]int{}
	for i, name := range configVariables {
		out[name] = i
	}
	return out
}

// verifyIDEqualValue processes identifier == (int | string) part of Condition.
func verifyIDEqualValue(expr *ast.BinaryExpr) (name string, value variableValue, err error) {
	id, ok := expr.X.(*ast.Ident)
	if !ok {
		err = errors.New("left operand of == must be identifier")
		return
	}
	name = id.Name

	val, ok := expr.Y.(*ast.BasicLit)
	if ok && val.Kind == token.INT {
		if i, parseErr := strconv.Atoi(val.Value); parseErr != nil {
			err = errors.New("right operand of == must be int or string value")
		} else {
			value.I = &i
		}
	} else if ok && val.Kind == token.STRING {
		// val.Value includes quotation marks, but we need just pure string.
		s := val.Value[1 : len(val.Value)-1]
		value.S = &s
	} else {
		err = errors.New("right operand of == must be int or string value")
	}
	return
}

// pythonToGoCondition converts Python code into valid Go code.
func pythonToGoCondition(pyCond string) (string, error) {
	// Isolate supported grammar is:
	//	expr ::= expr ( "or" | "and" ) expr
	//			| identifier "==" ( string | int )
	// and parentheses.
	// We convert this to equivalent Go expression by:
	//	* replacing all 'string' to "string"
	//  * replacing `and` and `or` to `&&` and `||` operators, respectively.
	// We work with runes to be safe against unicode.
	left := stringToRunes(pyCond)
	var err error
	goChunk := ""
	out := []string{}
	for len(left) > 0 {
		// Process non-string tokens till next string token.
		goChunk, left = pythonToGoNonString(left)
		out = append(out, goChunk)
		if len(left) != 0 {
			if goChunk, left, err = pythonToGoString(left); err != nil {
				return "", err
			}
			out = append(out, goChunk)
		}
	}
	return strings.Join(out, ""), nil
}

var rePythonAnd = regexp.MustCompile(`(\band\b)`)
var rePythonOr = regexp.MustCompile(`(\bor\b)`)

func pythonToGoNonString(left []rune) (string, []rune) {
	end := len(left)
	for i, r := range left {
		if r == '\'' || r == '"' {
			end = i
			break
		}
	}
	out := string(left[:end])
	out = rePythonAnd.ReplaceAllString(out, "&&")
	out = rePythonOr.ReplaceAllString(out, "||")
	return out, left[end:]
}

func pythonToGoString(left []rune) (string, []rune, error) {
	quoteRune := left[0]
	if quoteRune != '"' && quoteRune != '\'' {
		panic(fmt.Errorf("pythonToGoString must be called with ' or \" as first rune: %s", string(left)))
	}
	left = left[1:]
	escaped := false
	goRunes := []rune{'"'}
	for i, c := range left {
		if c == quoteRune && !escaped {
			goRunes = append(goRunes, '"')
			return string(goRunes), left[i+1:], nil
		}
		if c == '\'' && escaped {
			// Python allows "\'a", which is the same as "'a", but Go
			// doesn't allow this, so remove redundant escape before '.
			goRunes = goRunes[:len(goRunes)-1]
		} else if c == '"' && !escaped {
			// Either " is first char, or " is unescaped inside a string.
			goRunes = append(goRunes, '\\')
		}
		goRunes = append(goRunes, c)
		if c == '\\' {
			escaped = !escaped
		} else {
			escaped = false
		}
	}
	return string(goRunes), left, errors.New("failed to parse Condition string")
}

func (v *variables) isEmpty() bool {
	return len(v.Command) == 0 && len(v.Files) == 0 && v.ReadOnly == nil
}

func (v *variables) verify() error {
	if v.ReadOnly == nil || (0 <= *v.ReadOnly && *v.ReadOnly <= 2) {
		return nil
	}
	return errors.New("read_only must be 0, 1, 2, or undefined")
}

// configName defines a config as an ordered set of bound and unbound variable values.
type configName []variableValue

func (c configName) compare(rhs configName) int {
	// Bound value is less than unbound one.
	assert(len(c) == len(rhs))
	for i, l := range c {
		if r := l.compare(rhs[i]); r != 0 {
			return r
		}
	}
	return 0
}

func (c configName) Equals(o configName) bool {
	if len(c) != len(o) {
		return false
	}
	return c.compare(o) == 0
}

func (c configName) key() string {
	parts := make([]string, 0, len(c))
	for _, v := range c {
		if !v.isBound() {
			parts = append(parts, "∀")
		} else {
			parts = append(parts, "∃", v.String())
		}
	}
	return strings.Join(parts, "\x00")
}

type configPair struct {
	key   configName
	value *ConfigSettings
}

// configPairs implements interface for sort package sorting.
type configPairs []configPair

func (c configPairs) Len() int {
	return len(c)
}

func (c configPairs) Less(i, j int) bool {
	return c[i].key.compare(c[j].key) > 0
}

func (c configPairs) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
