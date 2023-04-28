package typescript

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicType_ToTypeScript_Success(t *testing.T) {
	assert.Equal(t, "boolean", Boolean.ToTypeScript())
	assert.Equal(t, "number", Number.ToTypeScript())
	assert.Equal(t, "string", String.ToTypeScript())
}

func TestLiteralType_ToTypeScript_Success(t *testing.T) {
	literalType := LiteralType{BasicType: Boolean, Literal: "true"}
	assert.Equal(t, "true", literalType.ToTypeScript())

	literalType = LiteralType{BasicType: Number, Literal: "123.45"}
	assert.Equal(t, "123.45", literalType.ToTypeScript())

	literalType = LiteralType{BasicType: String, Literal: "hello"}
	assert.Equal(t, `'hello'`, literalType.ToTypeScript())
}

func TestLiteralType_ToTypeScript_InvalidLiteral_Panics(t *testing.T) {
	assert.PanicsWithValue(t, `Invalid boolean literal: "maybe"`, func() {
		literalType := LiteralType{BasicType: Boolean, Literal: "maybe"}
		literalType.ToTypeScript()
	})

	assert.PanicsWithValue(t, `Invalid basic type: "faketype"`, func() {
		literalType := LiteralType{BasicType: BasicType("faketype"), Literal: "hello"}
		literalType.ToTypeScript()
	})
}

func TestArrayType_ToTypeScript_Sucess(t *testing.T) {
	arrayType := ArrayType{ItemsType: String}
	assert.Equal(t, "string[]", arrayType.ToTypeScript())

	arrayType = ArrayType{
		ItemsType: &UnionType{
			Types: []Type{String, Number},
		},
	}
	assert.Equal(t, "(string | number)[]", arrayType.ToTypeScript())
}

func TestMapType_ToTypeScript_Success(t *testing.T) {
	mapType := MapType{
		IndexType: String,
		ValueType: Number,
	}
	assert.Equal(t, "{ [key: string]: number }", mapType.ToTypeScript())
}

func TestMapType_ToTypeScript_InvalidIndexType_Panics(t *testing.T) {
	assert.PanicsWithValue(t, `TypeScript type "boolean" cannot be used as an index signature parameter type.`, func() {
		mapType := MapType{
			IndexType: Boolean,
			ValueType: Number,
		}
		mapType.ToTypeScript()
	})
}

func TestUnionType_ToTypeScript_Success(t *testing.T) {
	unionType := UnionType{
		Types: []Type{
			&LiteralType{BasicType: String, Literal: "up"},
			&LiteralType{BasicType: String, Literal: "right"},
			&LiteralType{BasicType: String, Literal: "down"},
			&LiteralType{BasicType: String, Literal: "left"},
		},
	}
	assert.Equal(t, `'up' | 'right' | 'down' | 'left'`, unionType.ToTypeScript())
}

func TestTypeAliasDeclaration_ToTypeScript_Success(t *testing.T) {
	unionType := UnionType{
		Types: []Type{
			&LiteralType{BasicType: String, Literal: "up"},
			&LiteralType{BasicType: String, Literal: "right"},
			&LiteralType{BasicType: String, Literal: "down"},
			&LiteralType{BasicType: String, Literal: "left"},
		},
	}

	typeAliasDeclaration := TypeAliasDeclaration{
		Identifier: "Direction",
		Type:       unionType,
	}
	assert.Equal(t, `export type Direction = 'up' | 'right' | 'down' | 'left';`, typeAliasDeclaration.ToTypeScript())

	typeAliasDeclaration.Namespace = "Foo"
	assert.Equal(t, `export namespace Foo { export type Direction = 'up' | 'right' | 'down' | 'left'; }`, typeAliasDeclaration.ToTypeScript())
}

func TestTypeAliasDeclaration_TypeReference_ReferenceReflectsChangesInDeclaration(t *testing.T) {
	typeAliasDeclaration := TypeAliasDeclaration{
		Identifier: "MyAlias",
		Type:       String,
	}

	typeReference := typeAliasDeclaration.TypeReference()
	assert.Equal(t, "MyAlias", typeReference.ToTypeScript())

	typeAliasDeclaration.Namespace = "Foo"
	assert.Equal(t, "Foo.MyAlias", typeReference.ToTypeScript())

	typeAliasDeclaration.Identifier = "AnotherAlias"
	assert.Equal(t, "Foo.AnotherAlias", typeReference.ToTypeScript())
}

func TestInterfaceDeclaration_ToTypeScript_Success(t *testing.T) {
	interfaceDeclaration := InterfaceDeclaration{
		Identifier: "Person",
		Properties: []PropertySignature{
			{
				Identifier: "Name",
				Type:       String,
			},
			{
				Identifier: "Age",
				Type:       Number,
				Optional:   true,
			},
		},
	}

	assert.Equal(t, `export interface Person {
	Name: string;
	Age?: number;
}`, interfaceDeclaration.ToTypeScript())

	interfaceDeclaration.Namespace = "Foo"
	assert.Equal(t, `export namespace Foo {
	export interface Person {
		Name: string;
		Age?: number;
	}
}`, interfaceDeclaration.ToTypeScript())
}

func TestInterfaceDeclaration_TypeReference_ReferenceReflectsChangesInDeclaration(t *testing.T) {
	interfaceDeclaration := InterfaceDeclaration{
		Identifier: "MyInterface",
	}

	typeReference := interfaceDeclaration.TypeReference()
	assert.Equal(t, "MyInterface", typeReference.ToTypeScript())

	interfaceDeclaration.Namespace = "Foo"
	assert.Equal(t, "Foo.MyInterface", typeReference.ToTypeScript())

	interfaceDeclaration.Identifier = "AnotherInterface"
	assert.Equal(t, "Foo.AnotherInterface", typeReference.ToTypeScript())
}
