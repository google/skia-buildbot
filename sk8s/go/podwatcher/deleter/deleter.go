package deleter

import "context"

// PodDeleter deletes pods with the given name.
type PodDeleter interface {
	Delete(context.Context, string) error
}
