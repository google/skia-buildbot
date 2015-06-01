package fail

import (
	"go.skia.org/infra/fuzzer/go/generator"
)

type CrashGenerator struct{}

func NewCrash() generator.Generator {
	return CrashGenerator{}
}

func (f CrashGenerator) Fuzz(g *generator.Writer) error {
	nullPointer := g.NewVariable("char*", "pointer_to_nowhere")
	g.AddStatement("%s = (char*) NULL", nullPointer)
	g.AddStatement("*%s = 'H'", nullPointer)

	return nil
}

func init() {
	generator.Register("crash", NewCrash)
}
