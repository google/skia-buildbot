package main

import (
	"context"
	"flag"

	"go.skia.org/infra/autoroll/go/rpc"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	target = flag.String("target", "http://localhost:8000", "RPC server.")
)

func main() {
	common.Init()

	ctx := context.Background()
	/*ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatal(err)
	}*/
	client := httputils.DefaultClientConfig(). /*.WithTokenSource(ts)*/ Client()
	rpcs := rpc.NewAutoRollRPCsJSONClient(*target, client)
	rollers, err := rpcs.Admin_GetRollers(ctx, &rpc.GetRollersRequest{})
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Errorf("%+v", rollers)
}
