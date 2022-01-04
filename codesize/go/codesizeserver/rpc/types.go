package rpc

import "go.skia.org/infra/codesize/go/bloaty"

type BloatyRPCResponse struct {
	// Rows represents the rows in the two-dimensional JavaScript array that the front-end passes to
	// google.visualization.arrayToDataTable().
	Rows []bloaty.TreeMapDataTableRow `json:"rows" go2ts:"ignorenil"`
}
