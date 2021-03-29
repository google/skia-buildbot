package ingestion_processors

import (
	"context"
	"sort"
	"strconv"

	"go.opencensus.io/trace"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/skerr"
)

const (
	lookupInBuildbucket = "lookup_bb"

	// These values are arbitrary guesses, roughly based on values observed
	// by previous implementation in production.
	maxQPS   = rate.Limit(10.0)
	maxBurst = 40
)

type LookupSystem interface {
	// Lookup returns the given CRS system, CL ID, PS Order or an error if it could not lookup
	// the CL information.
	Lookup(ctx context.Context, tjID string) (string, string, int, error)
}

func newBuildbucketLookupClient(client *buildbucket.Client) *bbLookupClient {
	return &bbLookupClient{
		client: client,
		rl:     rate.NewLimiter(maxQPS, maxBurst),
	}
}

type bbLookupClient struct {
	client *buildbucket.Client
	rl     *rate.Limiter
}

// Lookup takes the given Buildbucket ID and looks up the associated Gerrit CL. There may be more
// than one, so for now, it returns the one with the highest CL ID.
func (b *bbLookupClient) Lookup(ctx context.Context, tjID string) (string, string, int, error) {
	// Respect the rate limit.
	if err := b.rl.Wait(ctx); err != nil {
		return "", "", 0, skerr.Wrap(err)
	}
	buildID, err := strconv.ParseInt(tjID, 10, 64)
	if err != nil {
		return "", "", 0, skerr.Wrapf(err, "Invalid TryJob ID %q", tjID)
	}

	ctx, span := trace.StartSpan(ctx, "buildbucket_Lookup")
	defer span.End()
	build, err := b.client.GetBuild(ctx, buildID)
	if err != nil {
		return "", "", 0, skerr.Wrap(err)
	}
	cls := build.Input.GerritChanges
	if len(cls) == 0 {
		return "", "", 0, skerr.Fmt("Tryjob %s had no CLs associated with it", tjID)
	}
	sort.Slice(cls, func(i, j int) bool {
		return cls[i].Change > cls[j].Change
	})
	return "gerrit", strconv.FormatInt(cls[0].Change, 10), int(cls[0].Patchset), nil
}
