// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package svctool

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/logging"
)

type fileAndType struct {
	f *ast.File
	t *ast.TypeSpec
}

type parser struct {
	services     []*Service
	extraImports map[string]string

	fileSet *token.FileSet
	files   []*ast.File
	types   []string

	typeCache  map[string]fileAndType
	exprStrBuf bytes.Buffer
}

// parsePackage parses .go files and fills in p.files with the ASTs.
// Files must be in the same directory and have the same package name.
func (p *parser) parsePackage(fileNames []string) error {
	if len(fileNames) == 0 {
		return fmt.Errorf("fileNames is empty")
	}
	for i, name := range fileNames {
		if i > 0 && filepath.Dir(name) != filepath.Dir(fileNames[0]) {
			return fmt.Errorf("Go files belong to different directories")
		}
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		file, err := goparser.ParseFile(p.fileSet, name, nil, 0)
		if err != nil {
			return fmt.Errorf("parsing %s: %s", name, err)
		}
		if len(p.files) > 0 && file.Name.Name != p.files[0].Name.Name {
			return fmt.Errorf("Go files belong to different packages")
		}
		p.files = append(p.files, file)
	}
	if len(p.files) == 0 {
		return fmt.Errorf("no buildable Go files")
	}
	return nil
}

func (p *parser) resolveServices(c context.Context) error {
	for _, t := range p.types {
		svc, err := p.resolveService(c, t)
		if err != nil {
			return err
		}
		p.services = append(p.services, svc)
	}
	return nil
}

// recordImport extracts the package referenced by type expression typ,
// resolves its path and saves to p.extraImports.
func (p *parser) recordImport(f *ast.File, typ ast.Expr) error {
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}

	sel, ok := typ.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	pkgID, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	if _, ok := p.extraImports[pkgID.Name]; ok {
		return nil
	}

	path := ""
	for _, i := range f.Imports {
		if i.Name.Name == pkgID.Name {
			path = strings.Trim(i.Path.Value, `"`)
			break
		}
	}
	if path == "" {
		return fmt.Errorf("could not resolve package %s", pkgID.Name)
	}
	if p.extraImports == nil {
		p.extraImports = make(map[string]string)
	}
	p.extraImports[pkgID.Name] = path
	return nil
}

func (p *parser) resolveService(c context.Context, typeName string) (*Service, error) {
	file, typeSpec := p.findType(typeName)
	if typeSpec == nil {
		return nil, fmt.Errorf("type %s not found", typeName)
	}
	iface, ok := typeSpec.Type.(*ast.InterfaceType)
	if !ok {
		return nil, fmt.Errorf("%s is not an interface", typeName)
	}

	const suffix = "Server"
	if !strings.HasSuffix(typeName, suffix) {
		return nil, fmt.Errorf("expected type name %q to end with %q", typeName, suffix)
	}

	if iface.Methods == nil {
		return nil, fmt.Errorf("interface %q has no methods", typeName)
	}

	svc := &Service{
		TypeName: typeName,
		Node:     iface,
	}
	for _, m := range iface.Methods.List {
		signature, ok := m.Type.(*ast.FuncType)
		if !ok {
			ifaceName, err := p.exprString(m.Type)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("%s embeds %s; not supported", typeName, ifaceName)
		}

		name := m.Names[0].Name
		if signature.Params == nil {
			logging.Warningf(c, "%s.%s: no params", typeName, name)
			continue
		}
		params := signature.Params.List
		if len(params) != 2 {
			logging.Warningf(
				c,
				"%s.%s: param count is %d; expected 2",
				typeName, name, len(params))
			continue
		}
		if signature.Results == nil {
			logging.Warningf(c, "%s.%s: returns nothing", typeName, name)
			continue
		}
		results := signature.Results.List
		if len(results) != 2 {
			logging.Warningf(
				c,
				"%s.%s: return value count is %d; expected 2",
				typeName, name, len(results))
			continue
		}

		method := &Method{
			Node: m,
			Name: m.Names[0].Name,
		}

		if err := p.recordImport(file, params[1].Type); err != nil {
			return nil, err
		}
		var err error
		method.InputType, err = p.exprString(params[1].Type)
		if err != nil {
			return nil, err
		}

		if err := p.recordImport(file, results[0].Type); err != nil {
			return nil, err
		}
		method.OutputType, err = p.exprString(results[0].Type)
		if err != nil {
			return nil, err
		}
		svc.Methods = append(svc.Methods, method)
	}
	return svc, nil
}

func (p *parser) findType(name string) (*ast.File, *ast.TypeSpec) {
	if p.typeCache == nil {
		p.typeCache = map[string]fileAndType{}
	}
	if pair, ok := p.typeCache[name]; ok {
		return pair.f, pair.t
	}

	for _, f := range p.files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				p.typeCache[typeSpec.Name.Name] = fileAndType{f, typeSpec}
				if typeSpec.Name.Name == name {
					return f, typeSpec
				}
			}
		}
	}

	return nil, nil
}

// exprString renders expr to string.
func (p *parser) exprString(expr ast.Expr) (string, error) {
	p.exprStrBuf.Reset()
	err := printer.Fprint(&p.exprStrBuf, p.fileSet, expr)
	return p.exprStrBuf.String(), err
}
