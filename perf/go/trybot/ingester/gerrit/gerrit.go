package gerrit

import (
	"strconv"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/trybot"
	"go.skia.org/infra/perf/go/trybot/ingester"
)

// Gerrit implements trybot.Ingester for Gerrit code reviews.
type Gerrit struct {
	parser           *parser.Parser
	parseCounter     metrics2.Counter
	parseFailCounter metrics2.Counter
}

// New creates a new instance of Gerrit.
func New(parser *parser.Parser) *Gerrit {
	return &Gerrit{
		parser:           parser,
		parseCounter:     metrics2.GetCounter("perf_trybot_ingester_gerrit_parse_success", nil),
		parseFailCounter: metrics2.GetCounter("perf_trybot_ingester_gerrit_parse_failed", nil),
	}
}

// Start implements trybot.Ingester.
func (g *Gerrit) Start(files <-chan file.File) (<-chan trybot.TryFile, error) {
	ret := make(chan trybot.TryFile)
	go func() {
		for f := range files {
			issue, patchsetStr, err := g.parser.ParseTryBot(f)
			if err != nil {
				sklog.Warningf("Failed to parse: %s", err)
				g.parseFailCounter.Inc(1)
				continue
			}
			patchNumber, err := strconv.Atoi(patchsetStr)
			if err != nil {
				sklog.Warningf("Failed to parse: %s", err)
				g.parseFailCounter.Inc(1)
				continue
			}
			ret <- trybot.TryFile{
				CL:          issue,
				PatchNumber: patchNumber,
				Filename:    f.Name,
				Timestamp:   f.Created,
			}
			g.parseCounter.Inc(1)
		}
		close(ret)
	}()

	return ret, nil
}

// Assert that Gerrit implements ingester.Ingester.
var _ ingester.Ingester = (*Gerrit)(nil)
