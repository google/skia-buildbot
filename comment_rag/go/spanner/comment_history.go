package spanner

import (
	"strings"
)

// CLInfo stores the subject and description for a Changelist.
type CLInfo struct {
	Project       string   `sql:"project STRING(MAX) NOT NULL"`
	Repo          string   `sql:"repo STRING(MAX) NOT NULL"`
	ChangeID      int64    `sql:"change_id INT64 NOT NULL"`
	CLSubject     string   `sql:"cl_subject STRING(MAX)"`
	CLDescription string   `sql:"cl_description STRING(MAX)"`
	pk            struct{} `sql:"PRIMARY KEY (project, repo, change_id)"`
}

// CommentHistory stores the code review comment thread transcripts, diff snippets, analyses, and vector embeddings.
type CommentHistory struct {
	ID string `sql:"id STRING(MAX) PRIMARY KEY"`
	// Project represents the Gerrit instance GoB project (e.g., 'chromium' for chromium-review.googlesource.com).
	Project string `sql:"project STRING(MAX) NOT NULL"`
	// Repo represents the Git repository path within the project (e.g., 'chromium/src').
	Repo         string    `sql:"repo STRING(MAX)"`
	Category     string    `sql:"category STRING(MAX) NOT NULL"`
	ChangeID     int64     `sql:"change_id INT64 NOT NULL"`
	FilePath     string    `sql:"file_path STRING(MAX)"`
	CommentText  string    `sql:"comment_text STRING(MAX)"`
	CodeSnippet  string    `sql:"code_snippet STRING(MAX)"`
	Analysis     string    `sql:"analysis STRING(MAX)"`
	Embedding    []float32 `sql:"embedding ARRAY<FLOAT32>(vector_length=>768) NOT NULL"`
	fk           struct{}  `sql:"CONSTRAINT FK_CommentHistory_CLInfo FOREIGN KEY (project, repo, change_id) REFERENCES CLInfo (project, repo, change_id)"`
	embeddingIdx struct{}  `sql:"VECTOR INDEX CommentHistoryEmbeddingIndex (embedding) OPTIONS (distance_type='COSINE')"`
}

const (
	// CategoryIpcSecurity represents the Gerrit IPC Security Review category.
	CategoryIpcSecurity = "IPC_SECURITY"
)

// ValidCategories is the centralized package-level list of review categories supported by comment_rag.
var ValidCategories = []string{
	CategoryIpcSecurity,
}

// IsValidCategory checks if the requested category filter matches one of the centralized valid categories.
func IsValidCategory(cat string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(cat))
	for _, c := range ValidCategories {
		if c == normalized {
			return true
		}
	}
	return false
}
