package dummy

import "go.skia.org/infra/fuzzer/go/generator"

type DummyGenerator struct{}

func New() generator.Generator {
	return DummyGenerator{}
}

func (dr DummyGenerator) Fuzz(g *generator.Writer) error {
	g.AddStatement("SkPaint p")
	g.AddStatement("p.setColor(SK_ColorRED)")
	g.AddStatement("p.setAntiAlias(true)")
	g.AddStatement("p.setStyle(SkPaint::kStroke_Style)")
	g.AddStatement("p.setStrokeWidth(10)")
	g.AddStatement("canvas->drawLine(20, 20, 100, 100, p)")
	return nil
}

func init() {
	generator.Register("dummy", New)
}
