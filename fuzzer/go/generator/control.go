package generator

import (
	"fmt"

	"go.skia.org/infra/fuzzer/go/config"
)

func (g *Writer) AddLoop(reps int, unroll bool, generateBody func()) {
	if unroll {
		for i := 0; i < reps; i++ {
			generateBody()
		}
	} else {
		g.AddRaw(fmt.Sprintf("for (int i = 0 ; i < %d; i++) {", reps))
		g.Indent += config.Config.Fuzzer.Indentation
		generateBody()
		g.Indent -= config.Config.Fuzzer.Indentation
		g.AddRaw("}")
	}
}
