package roller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/revision"
)

func TestAutoRollerRolledPast(t *testing.T) {

	ctx := context.Background()
	r := &AutoRoller{}
	rev := func(id string) *revision.Revision {
		return &revision.Revision{Id: id}
	}
	r.lastRollRev = rev("0")
	r.nextRollRev = rev("1") // Pretend we're configured to roll one rev at a time.
	r.tipRev = rev("5")
	r.notRolledRevs = []*revision.Revision{
		rev("5"),
		rev("4"),
		rev("3"),
		rev("2"),
		rev("1"),
	}

	check := func(id string, expect bool) {
		got, err := r.RolledPast(ctx, &revision.Revision{Id: id})
		require.NoError(t, err)
		require.Equal(t, expect, got)
	}

	check("0", true)              // lastRollRev
	check("1", false)             // nextRollRev
	check("2", false)             // notRolledRev
	check("3", false)             // notRolledRev
	check("4", false)             // notRolledRev
	check("5", false)             // tipRev
	check("some other rev", true) // everything else
}
