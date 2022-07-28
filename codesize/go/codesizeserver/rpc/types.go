package rpc

import (
	"go.skia.org/infra/codesize/go/bloaty"
	"go.skia.org/infra/codesize/go/common"
	"go.skia.org/infra/codesize/go/store"
)

type BinaryRPCRequest struct {
	store.CommitOrPatchset
	BinaryName      string `json:"binary_name"`
	CompileTaskName string `json:"compile_task_name"`
}

type BinaryRPCResponse struct {
	Metadata common.BloatyOutputMetadata `json:"metadata"`

	// Rows represents the rows in the two-dimensional JavaScript array that the front-end passes to
	// google.visualization.arrayToDataTable().
	Rows []bloaty.TreeMapDataTableRow `json:"rows" go2ts:"ignorenil"`
}

type BinarySizeDiffRPCRequest struct {
	store.CommitOrPatchset
	BinaryName      string `json:"binary_name"`
	CompileTaskName string `json:"compile_task_name"`
}

type BinarySizeDiffRPCResponse struct {
	Metadata common.BloatyOutputMetadata `json:"metadata"`

	// RawDiff is the verbatim Bloaty size diff output in plain-text format.
	RawDiff string `json:"raw_diff"`
}

type MostRecentBinariesRPCResponse struct {
	Binaries []store.BinariesFromCommitOrPatchset `json:"binaries"`
}
