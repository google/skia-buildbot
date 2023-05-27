package backends

// TODO(seanmccullough): consolidate whatever we can with skia's existing RBE-CAS client library
// code in buildbot/go/cas, after we have the rest cabe's analyzer logic moved into this repo.

import (
	"context"

	"go.skia.org/infra/go/sklog"
	"google.golang.org/grpc"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
)

const (
	rbeServiceAddress = "remotebuildexecution.googleapis.com:443"
)

var (
	casInstances = []string{
		"projects/chrome-swarming/instances/default_instance",
		"projects/chromium-swarm/instances/default_instance",
		"projects/omnibot-swarming-server/instances/default_instance",
	}
)

// DialRBECAS will attempt to dial a set of RBE-CAS service backends for CABE.
// We create one Client for each RBE-CAS instance we need to connect to, each
// of which is Dialed separately. Returns either a map of CAS instance name to
// Client, or an error if any of the Clients could not be Dialed.
func DialRBECAS(ctx context.Context, dialOpts ...grpc.DialOption) (map[string]*rbeclient.Client, error) {
	creds, err := outboundGRPCCreds(ctx)
	if err != nil {
		sklog.Errorf("getting grpc creds: %v", err)
		return nil, err
	}
	dialParams := rbeclient.DialParams{
		Service:               rbeServiceAddress,
		UseApplicationDefault: true,
		DialOpts:              dialOpts,
	}
	perRPCCreds := &rbeclient.PerRPCCreds{
		Creds: *creds,
	}

	type connectResult struct {
		casInstance string
		rbeClient   *rbeclient.Client
		err         error
	}
	results := make(chan connectResult)

	for _, casInstance := range casInstances {
		casInstance := casInstance
		go func() {
			rbeClient, err := rbeclient.NewClient(ctx, casInstance, dialParams, perRPCCreds)
			results <- connectResult{
				casInstance,
				rbeClient,
				err,
			}
		}()
	}

	rbeClients := map[string]*rbeclient.Client{}

	for range casInstances {
		result := <-results
		if result.err != nil {
			// Don't just return here; that would leak any remaining goroutines trying to
			// send on chan results. So read everything from the channel first, then return
			// an error later, if there were any.
			err = result.err
		}
		rbeClients[result.casInstance] = result.rbeClient
	}

	return rbeClients, err
}
