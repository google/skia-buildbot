// TODO(b/500974820): Reuse types from `pinpoint/proto/v1/service.pb.go`.
package pinpoint

import "go.skia.org/infra/pinpoint/go/pinpoint/internal"

type (
	TryJobCreateRequest    = internal.TryJobCreateRequest
	BisectJobCreateRequest = internal.BisectJobCreateRequest
	CreatePinpointResponse = internal.CreatePinpointResponse
)

type (
	FetchJobStateRequest  = internal.FetchJobStateRequest
	FetchJobStateResponse = internal.FetchJobStateResponse
)

type (
	StateItem = internal.StateItem
	Change    = internal.Change
	Commit    = internal.Commit
)
