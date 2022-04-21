package mocks

//go:generate bazelisk run //:mockery   -- --name Calculator  --srcpkg=go.skia.org/infra/golden/go/diff --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name ImageSource  --srcpkg=go.skia.org/infra/golden/go/diff/worker --output ${PWD}
