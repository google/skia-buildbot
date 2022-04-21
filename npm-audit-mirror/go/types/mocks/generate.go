package mocks

//go:generate bazelisk run //:mockery   -- --name ChecksManager  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name Check  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name ProjectAudit  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name ProjectMirror  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name NpmDB  --srcpkg=go.skia.org/infra/npm-audit-mirror/go/types --output ${PWD}
