// Package trybot contains the endpoints for handling and serving the trybot
// pages.
package trybot

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// RequestKind is the type of data we are requesting to analyze on the
// trybot page. The request will be either for trybot data, or for a specific
// commit that's already landed.
type RequestKind string

const (
	// TryBot is a request to analyze the results from trybots.
	TryBot RequestKind = "trybot"

	// Commit is a request to analyze a commit that's already landed.
	Commit RequestKind = "commit"
)

// AllRequestKind is a list of all possible values of type RequestKind.
var AllRequestKind = []RequestKind{
	TryBot,
	Commit,
}

// TryBotRequest is the request the UI sends to retrieve data to analyze.
type TryBotRequest struct {
	Kind         RequestKind        `json:"kind"`
	CommitNumber types.CommitNumber `json:"cid"`
	CL           types.CL           `json:"cl"`
	Query        query.Query        `json:"query"`
}

// Result of the analysis for a single trace id.
type Result struct {
	// Params is the parsed trace id in params format.
	Params paramtools.Params `json:"params"`

	// See vec32.StdDevRatio for the definitions of Median, Lower, Upper and StdDevRation.
	Median      float32 `json:"median"`
	Lower       float32 `json:"lower"`
	Upper       float32 `json:"upper"`
	StdDevRatio float32 `json:"stddevRatio"`

	// The values for the trace at the last N commits, and either the value at
	// commit N+1, or a trybot result, depending on the RequestKind sent in
	// TryBotRequest.
	Values []float32 `json:"values"`
}

// TryBotResponse is the response sent to a TryBotRequest.
type TryBotResponse struct {
	Header  []dataframe.ColumnHeader
	Results []Result
}
