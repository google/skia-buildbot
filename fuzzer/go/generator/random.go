package generator

import (
	"fmt"
	"math/rand"

	"github.com/skia-dev/glog"
)

func RandInt(min, max int) int {
	return rand.Intn(max-min+1) + min
}

func RandIntStr(min, max int, immediate bool) string {
	if immediate {
		return fmt.Sprintf("%d", RandInt(min, max))
	} else {
		glog.Fatalf("RandIntStr Not implemented yet")
		return ""
	}
}

func RandFloat(min, max float32) float32 {
	return (max-min)*rand.Float32() + min
}

func RandAngle() float32 {
	return rand.Float32()
}

func RandBool() bool {
	return rand.Float32() > 0.5
}

func RandFloatStr(min, max float32, immediate bool) string {
	if immediate {
		return fmt.Sprintf("%f", RandFloat(min, max))
	} else {
		glog.Fatalf("RandFloatStr Not implemented yet")
		return ""
	}
}

func (g *Writer) RandAngleStr() string {
	return fmt.Sprintf("%f", RandAngle())
}

func (g *Writer) RandBoolStr() string {
	return fmt.Sprintf("%t", RandBool())
}

func (g *Writer) RandScalarStr() string {
	return RandFloatStr(g.FloatMin, g.FloatMax, true)
}

func (g *Writer) RandDirectionStr() string {
	directions := []string{"SkPath::kCW_Direction", "SkPath::kCCW_Direction"}
	idx := rand.Intn(len(directions))
	return directions[idx]
}

func (g *Writer) RandAddPathModeStr() string {
	modes := []string{"SkPath::kAppend_AddPathMode", "SkPath::kExtend_AddPathMode"}
	idx := rand.Intn(len(modes))
	return modes[idx]
}

func (g *Writer) AddRandScalarArray(name string, size int) string {
	array := g.NewArray("SkScalar", name, size)
	for i := 0; i < size; i++ {
		g.AddStatement("%s[%d] = %s", array, i, g.RandScalarStr())
	}
	return array
}

func (g *Writer) AddRandVectorArray(name string, size int) string {
	array := g.NewArray("SkVector", name, size)
	for i := 0; i < size; i++ {
		g.AddStatement("%s[%d].fX = %s", array, i, g.RandScalarStr())
		g.AddStatement("%s[%d].fY = %s", array, i, g.RandScalarStr())
	}
	return array
}
