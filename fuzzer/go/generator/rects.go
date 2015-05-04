package generator

type Rect struct {
	Name string
	G    *Writer
}

func (g *Writer) AddRect() Rect {
	rect := Rect{g.NewVariable("SkRect", "rect"), g}
	return rect
}

func (g *Writer) AddRandRect() Rect {
	rect := g.AddRect()
	rect.Rand()
	return rect
}

func (r *Rect) Rand() {
	r.G.AddStatement("%s.fLeft = %s", r.Name, r.G.RandScalarStr())
	r.G.AddStatement("%s.fRight = %s", r.Name, r.G.RandScalarStr())
	r.G.AddStatement("%s.fTop = %s", r.Name, r.G.RandScalarStr())
	r.G.AddStatement("%s.fBottom = %s", r.Name, r.G.RandScalarStr())
}
