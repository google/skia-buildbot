package issuesync

type IssueSync interface {
}

type RemoteRec struct {
	Title       string
	Description string
}

func New() IssueSync {
	return nil
}
