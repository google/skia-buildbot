package build

type FindBuildRequest struct {
	Request any
}

type FindBuildResponse struct {
	Response any
	BuildID  int64
}

type StartBuildRequest struct {
	Request any
}

type StartBuildResponse struct {
	Response any
}

type GetBuildArtifactRequest struct {
	BuildID int64
	Target  string
}

type GetBuildArtifactResponse struct {
	Response any
}

type CancelBuildRequest struct {
	BuildID int64
	Reason  string
}
