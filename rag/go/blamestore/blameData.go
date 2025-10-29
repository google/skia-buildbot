package blamestore

// LineBlame represents the blame for a single line.
type LineBlame struct {
	LineNumber int64
	CommitHash string
}

// FileBlame represents the blame information for a single file.
type FileBlame struct {
	FilePath   string
	FileHash   string
	Version    string
	CommitHash string
	LineBlames []*LineBlame
}
