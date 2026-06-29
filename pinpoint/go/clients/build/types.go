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
	Target  string
	BuildID int64
}

type GetBuildArtifactResponse struct {
	Response any
}

type CancelBuildRequest struct {
	Reason  string
	BuildID int64
}
