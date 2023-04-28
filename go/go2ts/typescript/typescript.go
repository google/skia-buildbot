// Package typescript provides primitives to represent TypeScript types, and type declarations.
//
// This package is completely decoupled from Go's reflect package. It provides a set of very simple
// primitives to build an AST-like representation of TypeScript types, and type declarations. Each
// primitive includes a ToTypeScript() method that can be used to recursively generate Go2TS's
// output TypeScript code.
//
// These primitives decouple Go2TS's reflection code from its code generation code, which makes
// Go2TS easier to debug and reason about, and provide a flexible foundation that will allow us to
// extend Go2TS in the future with support for additional types, new features, etc.
//
// The "root" type in this package is the TypeDeclaration interface, which is implemented by the
// InterfaceDeclaration and TypeAliasDeclaration structs.
//
// The names of the primitives in this package are loosely based on the names and terms used in the
// TypeScript AST. Recommended links:
//   - https://ts-ast-viewer.com/
//   - https://github.com/Microsoft/TypeScript/wiki/Using-the-Compiler-API
package typescript

import (
	"fmt"
	"strings"
)

// Type represents a TypeScript type.
type Type interface {
	// ToTypeScript returns a TypeScript expression that can be used as a type, or panics if the type
	// is invalid.
	ToTypeScript() string

	isType()
}

///////////////
// BasicType //
///////////////

// BasicType represents a TypeScript basic type supported by Go2TS.
//
// The "null" and "any" types are represented as basic types for simplicity.
type BasicType string

const (
	// Boolean represents the "boolean" TypeScript type.
	Boolean = BasicType("boolean")

	// Number represents the "number" TypeScript type.
	Number = BasicType("number")

	// String represents the "string" TypeScript type.
	String = BasicType("string")

	// Null represents the "null" TypeScript type.
	Null = BasicType("null")

	// Any represents the "null" TypeScript type.
	Any = BasicType("any")
)

// ToTypeScript implements the Type interface.
func (b BasicType) ToTypeScript() string { return string(b) }

// isType implements the Type interface.
func (b BasicType) isType() {}

var _ Type = (*BasicType)(nil)

/////////////////
// LiteralType //
/////////////////

// LiteralType represents a TypeScript literal type such as "hello", 123, true, etc.
//
// See https://www.typescriptlang.org/docs/handbook/literal-types.html.
type LiteralType struct {
	BasicType BasicType
	Literal   string
}

// ToTypeScript implements the Type interface.
func (l *LiteralType) ToTypeScript() string {
	switch l.BasicType {
	case Boolean:
		if l.Literal != "true" && l.Literal != "false" {
			panic(fmt.Sprintf(`Invalid boolean literal: %q`, l.Literal))
		}
		return l.Literal
	case Number:
		return l.Literal
	case String:
		return fmt.Sprintf(`'%s'`, l.Literal)
	}
	panic(fmt.Sprintf(`Invalid basic type: %q`, l.BasicType))
}

// isType implements the Type interface.
func (l *LiteralType) isType() {}

var _ Type = (*LiteralType)(nil)

///////////////
// ArrayType //
///////////////

// ArrayType represents a TypeScript array type such as string[], MyType[], etc.
type ArrayType struct {
	ItemsType Type
}

// ToTypeScript implements the Type interface.
func (a *ArrayType) ToTypeScript() string {
	fmtStr := "%s[]"
	if _, ok := a.ItemsType.(*UnionType); ok {
		fmtStr = "(%s)[]"
	}
	return fmt.Sprintf(fmtStr, a.ItemsType.ToTypeScript())
}

// isType implements the Type interface.
func (a *ArrayType) isType() {}

var _ Type = (*ArrayType)(nil)

/////////////
// MapType //
/////////////

// MapType represents a TypeScript type that describes an object used as a dictionary of key/value
// pairs, e.g. { [key: string]: MyStruct }.
type MapType struct {
	IndexType Type
	ValueType Type
}

// ToTypeScript implements the Type interface.
func (m *MapType) ToTypeScript() string {
	indexTypeToTS := m.IndexType.ToTypeScript()
	if indexTypeToTS != "number" && indexTypeToTS != "string" {
		panic(fmt.Sprintf("TypeScript type %q cannot be used as an index signature parameter type.", indexTypeToTS))
	}

	return fmt.Sprintf("{ [key: %s]: %s }", indexTypeToTS, m.ValueType.ToTypeScript())
}

// isType implements the Type interface.
func (m *MapType) isType() {}

var _ Type = (*MapType)(nil)

///////////////
// UnionType //
///////////////

// UnionType represents a TypeScript union type, e.g. 'up' | 'right' | 'down' | 'left'.
type UnionType struct {
	Types []Type
}

// ToTypeScript implements the Type interface.
func (u UnionType) ToTypeScript() string {
	tsTypes := []string{}
	for _, t := range u.Types {
		tsTypes = append(tsTypes, t.ToTypeScript())
	}
	return strings.Join(tsTypes, " | ")
}

// isType implements the Type interface.
func (u UnionType) isType() {}

var _ Type = (*UnionType)(nil)

///////////////////
// TypeReference //
///////////////////

// TypeReference represents a reference to a type declared via a TypeDeclaration (e.g. an interface,
// a type alias, etc.) and can be used anywhere a Type can be used.
type TypeReference struct {
	typeDeclaration TypeDeclaration
}

// ToTypeScript implements the Type interface.
func (t *TypeReference) ToTypeScript() string {
	return t.typeDeclaration.QualifiedName()
}

// isType implements the Type interface.
func (t *TypeReference) isType() {}

var _ Type = (*TypeReference)(nil)

/////////////////////
// TypeDeclaration //
/////////////////////

// TypeDeclaration represents a TypeScript type declaration, which can be an interface declaration,
// a type alias, etc.
type TypeDeclaration interface {
	// TypeReference returns a reference to the declared type.
	TypeReference() *TypeReference

	// QualifiedName returns the qualified name of the declared type, e.g. MyNamespace.MyType.
	QualifiedName() string

	// ToTypeScript converts the TypeDeclaration to valid TypeScript.
	ToTypeScript() string

	isTypeDeclaration()
}

//////////////////////////
// TypeAliasDeclaration //
//////////////////////////

// TypeAliasDeclaration represents a TypeScript type alias declaration, e.g. type Color = string.
type TypeAliasDeclaration struct {
	// Namespace is the namespace that the type alias belongs to, or empty for the global namespace.
	Namespace  string
	Identifier string
	Type       Type
}

// TypeReference implements the TypeDeclaration interface.
func (a *TypeAliasDeclaration) TypeReference() *TypeReference {
	return &TypeReference{
		typeDeclaration: a,
	}
}

// QualifiedName implements the TypeDeclaration interface.
func (a *TypeAliasDeclaration) QualifiedName() string {
	return makeQualifiedName(a.Namespace, a.Identifier)
}

// ToTypeScript implements the TypeDeclaration interface.
func (a *TypeAliasDeclaration) ToTypeScript() string {
	if a.Namespace == "" {
		return fmt.Sprintf("export type %s = %s;", a.Identifier, a.Type.ToTypeScript())
	}
	return fmt.Sprintf("export namespace %s { export type %s = %s; }", a.Namespace, a.Identifier, a.Type.ToTypeScript())
}

// isTypeDeclaration implements the TypeDeclaration interface.
func (a *TypeAliasDeclaration) isTypeDeclaration() {}

var _ TypeDeclaration = (*TypeAliasDeclaration)(nil)

//////////////////////////
// InterfaceDeclaration //
//////////////////////////

// PropertySignature represents a property signature of a TypeScript interface declaration.
type PropertySignature struct {
	Identifier string
	Type       Type
	Optional   bool
}

// ToTypeScript converts the PropertySignature to a valid TypeScript interface property declaration.
func (p *PropertySignature) ToTypeScript() string {
	optionalString := ""
	if p.Optional {
		optionalString = "?"
	}
	return fmt.Sprintf("%s%s: %s;", p.Identifier, optionalString, p.Type.ToTypeScript())
}

// InterfaceDeclaration represents a TypeScript interface declaration.
type InterfaceDeclaration struct {
	// Namespace is the namespace that the interface belongs to, or empty for the global namespace.
	Namespace  string
	Identifier string
	Properties []PropertySignature
}

// TypeReference implements the TypeDeclaration interface.
func (i *InterfaceDeclaration) TypeReference() *TypeReference {
	return &TypeReference{
		typeDeclaration: i,
	}
}

// QualifiedName implements the TypeDeclaration interface.
func (i *InterfaceDeclaration) QualifiedName() string {
	return makeQualifiedName(i.Namespace, i.Identifier)
}

// ToTypeScript implements the TypeDeclaration interface.
func (i *InterfaceDeclaration) ToTypeScript() string {
	var sb strings.Builder

	namespaced := i.Namespace != ""
	interfaceIndentation := ""
	propertyIndentation := "\t"

	if namespaced {
		sb.WriteString(fmt.Sprintf("export namespace %s {\n", i.Namespace))
		interfaceIndentation = "\t"
		propertyIndentation = "\t\t"
	}

	sb.WriteString(interfaceIndentation)
	sb.WriteString(fmt.Sprintf("export interface %s {\n", i.Identifier))

	for _, prop := range i.Properties {
		sb.WriteString(propertyIndentation)
		sb.WriteString(prop.ToTypeScript())
		sb.WriteString("\n")
	}

	sb.WriteString(interfaceIndentation)
	sb.WriteString("}")

	if namespaced {
		sb.WriteString("\n}")
	}

	return sb.String()
}

// isTypeDeclaration implements the TypeDeclaration interface.
func (i *InterfaceDeclaration) isTypeDeclaration() {}

var _ TypeDeclaration = (*InterfaceDeclaration)(nil)

///////////////////////
// Utility functions //
///////////////////////

// makeQualifiedName returns a qualified TypeScript type name given a namespace and an identifier.
// If the namespace is the empty string, the type is assumed to be declared in the global namespace.
// Does not support nested namespaces.
func makeQualifiedName(namespace, identifier string) string {
	if namespace != "" {
		return fmt.Sprintf("%s.%s", namespace, identifier)
	}
	return identifier
}
