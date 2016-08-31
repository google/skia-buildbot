// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package svctool implements svcmux/svcdec tools command line parsing
package svctool

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/logging/gologger"
)

// Service contains the result of parsing the generated code for a pRPC service.
type Service struct {
	TypeName string
	Node     *ast.InterfaceType
	Methods  []*Method
}

type Method struct {
	Name       string
	Node       *ast.Field
	InputType  string
	OutputType string
}

type Import struct {
	Name string
	Path string
}

// Tool is a helper class for svcmux and svcdec.
type Tool struct {
	// Name of the tool, e.g. "svcmux" or "svcdec".
	Name string
	// OutputFilenameSuffix is the suffix of generated file names,
	// e.g. "mux" or "dec" for foo_mux.go or foo_dec.go.
	OutputFilenameSuffix string

	// Set by ParseArgs from command-line arguments.

	// Types are type names from the Go package defined by Dir or FileNames.
	Types []string
	// Output is the base name for the output file.
	Output string
	// Dir is a Go package's directory.
	Dir string
	// FileNames is a list of source files from a single Go package.
	FileNames []string
}

func (t *Tool) usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", t.Name)
	fmt.Fprintf(os.Stderr, "\t%s [flags] -type T [directory]\n", t.Name)
	fmt.Fprintf(os.Stderr, "\t%s [flags] -type T files... # Must be a single package\n", t.Name)
	flag.PrintDefaults()
}

func (t *Tool) parseFlags(args []string) []string {
	var flags = flag.NewFlagSet(t.Name, flag.ExitOnError)
	typeFlag := flags.String("type", "", "comma-separated list of type names; must be set")
	flags.StringVar(&t.Output, "output", "", "output file name; default <type>_string.go")
	flags.Usage = t.usage
	flags.Parse(args)

	splitTypes := strings.Split(*typeFlag, ",")
	t.Types = make([]string, 0, len(splitTypes))
	for _, typ := range splitTypes {
		typ = strings.TrimSpace(typ)
		if typ != "" {
			t.Types = append(t.Types, typ)
		}
	}
	if len(t.Types) == 0 {
		fmt.Fprintln(os.Stderr, "type is not specified")
		flags.Usage()
		os.Exit(2)
	}
	return flags.Args()
}

// ParseArgs parses command arguments. Exits if they are invalid.
func (t *Tool) ParseArgs(args []string) {
	args = t.parseFlags(args)

	switch len(args) {
	case 0:
		args = []string{"."}
		fallthrough

	case 1:
		info, err := os.Stat(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if info.IsDir() {
			t.Dir = args[0]
			t.FileNames, err = goFilesIn(args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			break
		}
		fallthrough

	default:
		t.Dir = filepath.Dir(args[0])
		t.FileNames = args
	}
}

// GeneratorArgs is passed to the function responsible for generating files.
type GeneratorArgs struct {
	PackageName  string
	Services     []*Service
	ExtraImports []Import
	Out          io.Writer
}
type Generator func(c context.Context, a *GeneratorArgs) error

// importSorted converts a map name -> path to []Import sorted by name.
func importSorted(imports map[string]string) []Import {
	names := make([]string, 0, len(imports))
	for n := range imports {
		names = append(names, n)
	}
	sort.Strings(names)
	result := make([]Import, len(names))
	for i, n := range names {
		result[i] = Import{n, imports[n]}
	}
	return result
}

// Run parses Go files and generates a new file using f.
func (t *Tool) Run(c context.Context, f Generator) error {
	// Validate arguments.
	if len(t.FileNames) == 0 {
		return fmt.Errorf("files not specified")
	}
	if len(t.Types) == 0 {
		return fmt.Errorf("types not specified")
	}

	// Determine output file name.
	outputName := t.Output
	if outputName == "" {
		if t.Dir == "" {
			return fmt.Errorf("neither output not dir are specified")
		}
		baseName := fmt.Sprintf("%s_%s.go", t.Types[0], t.OutputFilenameSuffix)
		outputName = filepath.Join(t.Dir, strings.ToLower(baseName))
	}

	// Parse Go files and resolve specified types.
	p := &parser{
		fileSet: token.NewFileSet(),
		types:   t.Types,
	}
	if err := p.parsePackage(t.FileNames); err != nil {
		return fmt.Errorf("could not parse .go files: %s", err)
	}
	if err := p.resolveServices(c); err != nil {
		return err
	}

	// Run the generator.
	var buf bytes.Buffer
	genArgs := &GeneratorArgs{
		PackageName:  p.files[0].Name.Name,
		Services:     p.services,
		ExtraImports: importSorted(p.extraImports),
		Out:          &buf,
	}
	if err := f(c, genArgs); err != nil {
		return err
	}

	// Format the output.
	src, err := format.Source(buf.Bytes())
	if err != nil {
		println(buf.String())
		return fmt.Errorf("gofmt: %s", err)
	}

	// Write to file.
	return ioutil.WriteFile(outputName, src, 0644)
}

// Main does some setup (arg parsing, logging), calls t.Run, prints any errors
// and exits.
func (t *Tool) Main(args []string, f Generator) {
	c := gologger.StdConfig.Use(context.Background())
	t.ParseArgs(args)

	if err := t.Run(c, f); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

// goFilesIn lists .go files in dir.
func goFilesIn(dir string) ([]string, error) {
	pkg, err := build.ImportDir(dir, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot process directory %s: %s", dir, err)
	}
	var names []string
	names = append(names, pkg.GoFiles...)
	names = append(names, pkg.CgoFiles...)
	names = prefixDirectory(dir, names)
	return names, nil
}

// prefixDirectory places the directory name on the beginning of each name in the list.
func prefixDirectory(directory string, names []string) []string {
	if directory == "." {
		return names
	}
	ret := make([]string, len(names))
	for i, name := range names {
		ret[i] = filepath.Join(directory, name)
	}
	return ret
}
