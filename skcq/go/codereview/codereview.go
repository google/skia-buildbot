package codereview

// Defines a generic interface used by the different code-review frameworks.

// After this is done look at autoroller codereview framework as well.
type CodeReview interface {
	Search() string

	// Have this return something like CodeReviewChange and then can pass this around below.
	GetDetails(cl int) string

	AddComment(cl int, comment string) string

	UpdateLabel(cl int) string

	Submit() string
}

// Extract this into it's own module under codereview called gerrit (also a mock one?)
