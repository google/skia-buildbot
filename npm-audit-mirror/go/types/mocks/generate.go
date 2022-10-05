package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name ChecksManager  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Check  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name ProjectAudit  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name ProjectMirror  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name NpmDB  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
