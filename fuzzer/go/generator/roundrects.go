package generator

type RRect struct {
	Name string
	G    *Writer
}

func (g *Writer) AddRRect() RRect {
	rrect := RRect{g.NewVariable("SkRRect", "rrect"), g}
	return rrect
}

func (g *Writer) AddRandRRect() RRect {
	rrect := g.AddRRect()
	rrect.Rand()
	return rrect
}

func (rr *RRect) Rand() {
	const (
		kSetEmpty       = 0
		kSetRect        = 1
		kSetOval        = 2
		kSetRectXY      = 3
		kSetNinePatch   = 4
		kSetRectRadii   = 5
		kMAX_RRECT_RAND = 6
	)

	rrectType := RandInt(0, kMAX_RRECT_RAND-1)
	if rrectType == kSetEmpty {
		rr.G.AddStatement("%s.setEmpty()", rr.Name)
	} else if rrectType == kSetRect {
		rect := rr.G.AddRandRect()
		rr.G.AddStatement("%s.setRect(%s)", rr.Name, rect.Name)
	} else if rrectType == kSetOval {
		oval := rr.G.AddRandRect()
		rr.G.AddStatement("%s.setOval(%s)", rr.Name, oval.Name)
	} else if rrectType == kSetRectXY {
		rect := rr.G.AddRandRect()
		rr.G.AddStatement("%s.setRectXY(%s, %s, %s)", rr.Name, rect.Name, rr.G.RandScalarStr(), rr.G.RandScalarStr())
	} else if rrectType == kSetNinePatch {
		rect := rr.G.AddRandRect()
		rr.G.AddStatement("%s.setNinePatch(%s, %s, %s, %s, %s)", rr.Name, rect.Name, rr.G.RandScalarStr(), rr.G.RandScalarStr(), rr.G.RandScalarStr(), rr.G.RandScalarStr())
	} else if rrectType == kSetRectRadii {
		rect := rr.G.AddRandRect()
		radii := rr.G.AddRandVectorArray("radii", 4)
		rr.G.AddStatement("%s.setRectRadii(%s, %s)", rr.Name, rect.Name, radii)
	}
}
