package generator

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/dgryski/go-discreterand"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
)

// Writer is a set of helper methods that we use to build up
// valid C code.
type Writer struct {
	Indent             int
	CodeLines          []string
	Counter            int
	Config             config.Generator
	FloatMin, FloatMax float32
}

func NewWriter(c config.Generator) *Writer {
	return &Writer{Indent: 2, Config: c, FloatMin: 10, FloatMax: 200}
}

func (g *Writer) AddRaw(line string) {
	g.CodeLines = append(g.CodeLines, strings.Repeat(" ", g.Indent)+line)
}

func (g *Writer) AddStatement(format string, args ...interface{}) {
	g.AddRaw(fmt.Sprintf(format, args...) + ";")
}

func (g *Writer) GetCode() string {
	ret := strings.Join(g.CodeLines, "\n")
	return ret
}

var (
	pluginNames        []string
	sampler            discreterand.AliasTable
	samplerInitialized bool = false
)

// initializeSampler sets up the discrete probability distribution used to select a plugin
func initializeSampler() {
	var probs []float64
	sum := 0.0
	for k := range config.Config.Generators {
		pluginNames = append(pluginNames, k)
		probs = append(probs, float64(config.Config.Generators[k].Weight))
		sum = sum + float64(config.Config.Generators[k].Weight)
	}
	for i := range probs {
		probs[i] /= sum
	}

	sampler = discreterand.NewAlias(probs, rand.NewSource(0))
}

// Fuzz selects a random fuzz generator, uses it to make the
// fuzz code, and returns the source to the caller.
func Fuzz() (string, error) {
	if samplerInitialized == false {
		samplerInitialized = true
		initializeSampler()
	}
	idx := sampler.Next()
	pluginName := pluginNames[idx]
	plugin := Constructor(pluginName)()

	g := NewWriter(config.Config.Generators[pluginName])
	e := plugin.Fuzz(g)
	if e != nil {
		glog.Fatalf("Couldn't generate random code: %s", e)
		return "", e
	}

	code := g.GetCode()
	if config.Config.Generators[pluginName].Debug {
		glog.Infof("%s", code)
	}

	return code, nil
}

var constructors map[string]func() Generator = make(map[string]func() Generator)

func Register(name string, f func() Generator) {
	constructors[name] = f
}

func Constructor(name string) func() Generator {
	constructor := constructors[name]
	if constructor == nil {
		glog.Fatalf("Not a registered randomizer name: %s", name)
		return func() Generator { return nil }
	}
	return constructor
}

type Generator interface {
	Fuzz(g *Writer) (err error)
}
