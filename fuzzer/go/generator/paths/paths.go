package paths

import (
	"fmt"

	"go.skia.org/infra/fuzzer/go/generator"
)

type PathGenerator struct{}

func New() generator.Generator {
	return PathGenerator{}
}

func (pr PathGenerator) Fuzz(g *generator.Writer) error {
	paint := g.AddPaint()
	matrix := g.AddMatrix()
	//	clip := g.AddPath()
	path := g.AddPath()

	g.AddLoop(10, true, func() {
		paint.Rand()
		matrix.Rand()

		depthLimit := generator.RandInt(1, 2)
		contourCount := generator.RandInt(1, 4)
		segmentLimit := generator.RandInt(1, 8)
		// clip.Rand(depthLimit, contourCount, segmentLimit)

		depthLimit = generator.RandInt(1, 2)
		contourCount = generator.RandInt(1, 2)
		segmentLimit = generator.RandInt(1, 2)
		path.Rand(depthLimit, contourCount, segmentLimit)

		g.AddStatement(fmt.Sprintf("canvas->setMatrix(%s)", matrix.Name))
		g.AddStatement(fmt.Sprintf("canvas->drawPath(%s,%s)", path.Name, paint.Name))

	})
	return nil
}

func init() {
	generator.Register("paths", New)
}
