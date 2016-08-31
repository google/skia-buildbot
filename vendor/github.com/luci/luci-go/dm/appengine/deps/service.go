// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"bytes"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const ek = logging.ErrorKey

type deps struct{}

var _ dm.DepsServer = (*deps)(nil)

func depsServerPrelude(c context.Context, methodName string, req proto.Message) (context.Context, error) {
	// Many of the DM request messages can be Normalize'd. This checks them for
	// basic validity and normalizes cases where multiple representations can mean
	// the same thing so that the service handlers only need to check for the
	// canonical representation.
	if norm, ok := req.(interface {
		Normalize() error
	}); ok {
		if err := norm.Normalize(); err != nil {
			return nil, grpcutil.Annotate(err, codes.InvalidArgument).Reason("invalid request").Err()
		}
	}
	return c, nil
}

const postludeDebugEnvvar = "DUMP_ALL_STACKS"

var postludeOmitCodes = map[codes.Code]struct{}{
	codes.OK:                 {},
	codes.Unauthenticated:    {},
	codes.AlreadyExists:      {},
	codes.FailedPrecondition: {},
	codes.InvalidArgument:    {},
	codes.NotFound:           {},
	codes.OutOfRange:         {},
	codes.Aborted:            {},
}

func depsServerPostlude(c context.Context, methodName string, rsp proto.Message, err error) error {
	retErr := grpcutil.ToGRPCErr(err)
	if err != nil {
		code := codes.OK
		_, printStack := os.LookupEnv(postludeDebugEnvvar)
		if !printStack {
			code = grpc.Code(retErr)
			_, omitStack := postludeOmitCodes[code]
			printStack = !omitStack
		}
		if printStack {
			buf := &bytes.Buffer{}
			errors.RenderStack(err).DumpTo(buf)
			logging.Errorf(c, "%s", buf.String())
		} else {
			logging.Infof(c, "returning gRPC code: %s", code)
		}
	}
	return retErr
}

func newDecoratedDeps() dm.DepsServer {
	return &dm.DecoratedDeps{
		Service:  &deps{},
		Prelude:  depsServerPrelude,
		Postlude: depsServerPostlude,
	}
}

// RegisterDepsServer registers an implementation of the dm.DepsServer with
// the provided Registrar.
func RegisterDepsServer(svr prpc.Registrar) {
	dm.RegisterDepsServer(svr, newDecoratedDeps())
}

// tumbleNow will run the mutation immediately, converting any non grpc errors
// to codes.Internal.
func tumbleNow(c context.Context, m tumble.Mutation) error {
	err := tumble.RunMutation(c, m)
	if grpc.Code(err) == codes.Unknown {
		logging.WithError(err).Errorf(c, "unknown error while applying mutation %v", m)
		err = grpcutil.Internal
	}
	logging.Fields{"root": m.Root(c)}.Infof(c, "tumbleNow success")
	return err
}
