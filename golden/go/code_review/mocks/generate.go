package mocks

//go:generate bazelisk run //:mockery   -- --name Client  --srcpkg=go.skia.org/infra/golden/go/code_review --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name ChangelistLandedUpdater  --srcpkg=go.skia.org/infra/golden/go/code_review --output ${PWD}
