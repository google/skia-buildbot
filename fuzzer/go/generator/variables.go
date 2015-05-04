package generator

import (
	"fmt"
)

func (g *Writer) NewVariable(vartype, name string) string {
	varname := fmt.Sprintf("%s_%d", name, g.Counter)
	g.AddStatement("%s %s", vartype, varname)
	g.Counter += 1
	return varname
}

func (g *Writer) NewArray(vartype, name string, size int) string {
	varname := fmt.Sprintf("%s_%d", name, g.Counter)
	g.AddStatement("%s %s[%d]", vartype, varname, size)
	g.Counter += 1
	return varname
}
