// Package results defines the interface for loading trybot results.
//
// Several types in this file have TryBot prefixes because they are exported via
// go2ts and this avoids name conflicts in TypeScript.
package results

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// Kind is the type of results we are requesting. The request will be
// either for trybot data, or for a specific commit that's already landed.
type Kind string

const (
	// TryBot is a request to analyze the results from trybots.
	TryBot Kind = "trybot"

	// Commit is a request to analyze a commit that's already landed.
	Commit Kind = "commit"
)

// AllRequestKind is a list of all possible values of type RequestKind.
var AllRequestKind = []Kind{
	TryBot,
	Commit,
}

// TryBotRequest is the request the UI sends to retrieve data to analyze.
type TryBotRequest struct {
	Kind Kind `json:"kind"`

	// CL is the ID of the changelist to analyze. Only use if Kind is TryBot.
	CL types.CL `json:"cl"`

	// CommitNumber is the commit to analyze. Only use if Kind is Commit.
	CommitNumber types.CommitNumber `json:"cid"`

	// Query is a query to select the set of traces to analys. Only used if Kind is Commit.
	Query string `json:"query"`
}

// TryBotResult of the analysis for a single trace id.
type TryBotResult struct {
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
	Results []TryBotResult
}

// Loader returns the data for the given TryBotRequest.
type Loader interface {
	// Load the TryBot results for the given TryBotRequest.
	Load(TryBotRequest) (TryBotResponse, error)
}
