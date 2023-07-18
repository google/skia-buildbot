// Package rubberstamper contains some utilities for having the RubberStamper bot automatically
// submit a CL. See skbug.com/12124 for more details.
// These utilities are not in the gerrit package to avoid adding the heavy dependencies of that
// package. We choose not to use depot_tools, as that can be a pain to deploy on all the places
// that would need to use the rubberstamper.
package rubberstamper

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"go.skia.org/infra/go/git"
)

const (
	// PushRequestAutoSubmit is the destination of a git push that will upload the given branch
	// to Gerrit and have RubberStamper +1 it and submit it.
	PushRequestAutoSubmit =
	// We want to push our local HEAD to a special Gerrit branch "refs/for/main". This creates a CL.
	// https://gerrit-review.googlesource.com/Documentation/user-upload.html#push_create
	"HEAD:refs/for/" + git.MainBranch + "%" +
		// We can provide options to Gerrit when pushing by including them as a comma seperated list
		// after a percent sign. We want to have the rubber stamper service be the one reviewer and
		// Auto Submit enabled so it can land the commit automatically.
		// https://gerrit-review.googlesource.com/Documentation/user-upload.html#push_options
		"notify=OWNER_REVIEWERS,l=Auto-Submit+1,r=" + RubberStamperUser

	// See skbug.com/12124 and go/rubber-stamper-user-guide for more on this user.
	RubberStamperUser = "rubber-stamper@appspot.gserviceaccount.com"

	// entropyBytes is how many random bytes to read in order to create a probabilistically unique
	// changelist ID. 256^100 seems like a reasonable amount of states.
	entropyBytes = 100
)

// RandomChangeID generates a probabilistically unique Gerrit ChangeId from the default rand source
func RandomChangeID(ctx context.Context) string {
	if id := ctx.Value(contextKey); id != nil {
		switch v := id.(type) {
		case string:
			return v
		default:
			panic(fmt.Sprintf("Unknown value for contextKey: %v", v))
		}
	}
	// In a normal flow (using depot_tools), the ChangeId is computed programmatically from many
	// inputs and then run through a SHA-1 hash. That does not appear to be used to validate
	// anything, rather just a way to deterministically compute something unique.
	// https://gerrit-review.googlesource.com/Documentation/user-changeid.html
	// https://gerrit-review.googlesource.com/Documentation/cmd-hook-commit-msg.html
	// Therefore, we randomly generate some bytes, hash them, and use that as our hopefully unique
	// ChangeId.
	b := make([]byte, entropyBytes)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	h := sha1.Sum(b)
	// Gerrit prepends the generated ChangeId with an uppercase I as a convention.
	return "Change-Id: I" + hex.EncodeToString(h[:])
}

type contextKeyType string

// ContextKey is used by tests to make the time deterministic.
//
// That is, in a test, you can write a value into a context to use as the return
// value of Now().
//
//	var mockTime = time.Unix(0, 12).UTC()
//	ctx = context.WithValue(ctx, now.ContextKey, mockTime)
//
// The value set can also be a function that returns a time.Time.
//
//	   var monotonicTime int64 = 0
//	   var mockTimeProvider = func() time.Time {
//	     monotonicTime += 1
//		    return time.Unix(monotonicTime, 0).UTC()
//	   }
//	   ctx = context.WithValue(ctx, now.ContextKey, now.NowProvider(mockTimeProvider))
const contextKey contextKeyType = "deterministicChangeID"

func WithDeterministicChangeID(ctx context.Context, changeID string) context.Context {
	return context.WithValue(ctx, contextKey, changeID)
}
