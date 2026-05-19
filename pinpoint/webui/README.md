# Pinpoint Web UI

This directory contains the Angular 21 Single Page Application for the Pinpoint
service. The application is built and packaged using Bazel.

## Architecture and Build Process

The application is built using the standard Angular CLI (`ng build`) executed
within a Bazel build action.

- **Bazel Build Target**: The `//pinpoint/webui:bundle` target runs
  `ng build` using the `js_run_binary` rule.
- **Builder**: The application uses the esbuild-based Angular application
  builder configured in `angular.json`.
- **TypeScript Configuration**: The application uses its own `tsconfig.app.json`
  configured for ES2022 module resolution.
- **Node Dependencies**: The build action receives direct dependencies from the
  workspace root `node_modules`.
- **Packaging**: The build output directory is packaged into a tarball
  (`pkg.tar`) using the `pkg_tar` rule. This tarball is used to inject the static
  files into the Pinpoint Web UI Docker container.

## Serving the Application

The compiled frontend files are served by the Pinpoint Go web server located at
`//pinpoint/go/webui`.

- **Static Files**: The Go server reads static resources (JavaScript, HTML, CSS)
  from a directory specified by the `--resources_dir` flag.
- **Routing Fallback**: To support client-side Path Routing (URLs without `#`),
  the Go server must distinguish between actual static assets and application
  routes. It does this by checking if the requested file exists on disk:
  - **Asset Requests**: If the file exists (e.g., `/main.js`), the server
    returns it. This ensures the browser successfully downloads the compiled code
    required to run the application.
  - **Route Requests**: If the file does not exist (e.g., `/new`), the server
    returns `index.html`. The Angular application then initializes in the browser
    and navigates to the route.

## Running the Server Locally

To build the frontend assets and execute the integrated Go web server locally
from the root of the workspace:

```bash
bazelisk run //pinpoint/go/webui
```

The web server directly hosts the frontend application from the active Bazel
runfiles tree. Open your browser to `http://localhost:8000`.

## Local Development with Live Reload

To develop the user interface with real-time updates, use the Angular CLI
development server (`ng serve`). This server reloads the application
automatically when you save changes to source files.

Because the frontend fetches job data from backend APIs, you need to run the Go
server to handle API requests.

### 1. Start the Backend API Server

In terminal 1, run the Go web server on port 8000:

```bash
bazelisk run //pinpoint/go/webui
```

### 2. Start the Frontend Development Server

In terminal 2, navigate to the frontend directory and run the Angular
development server on port 4200:

```bash
cd pinpoint/webui
npx ng serve
```

### 3. Open Application

Open your browser to `http://localhost:4200`.

- The Angular CLI uses `proxy.local.json` to route requests starting with
  `/pinpoint` and `/api` to the Go server on port 8000.
- Changes to `.ts` or `.html` files update the browser immediately.

### Local User Testing (Fake Auth)

When developing locally, the Google auth headers (like `X-Webauth-User`) are not
present. To simulate a logged-in user (e.g. for testing the header element or
profile popup), the Angular CLI dev-server proxy is configured in
`proxy.local.json` to inject a fake user header:

```json
"headers": {
  "Grpc-Metadata-X-Webauth-User": "test-user@google.com"
}
```

- **Default User**: The default user is `test-user@google.com`.
- **Changing User**: To test with a different email address, edit the value of
  `"Grpc-Metadata-X-Webauth-User"` in `pinpoint/webui/proxy.local.json` and
  restart the dev server.
- **Anonymous Mode**: To test the application as an anonymous/logged-out user,
  remove the `"headers"` block or the `"Grpc-Metadata-X-Webauth-User"` entry
  from `pinpoint/webui/proxy.local.json` and restart the dev server.

Note: if you modify `test-user@google.com`, revert it before merging.

## Running Unit Tests

Unit tests for the Web UI components are built and executed in a headless
browser via Karma + Mocha under Bazel.

To run all Web UI tests:

```bash
bazelisk test //pinpoint/webui/...
```

To run a specific component's tests (e.g., the Header Component):

```bash
bazelisk test //pinpoint/webui/app/header:header.component_test
```

## Regenerating TypeScript Types from Protobuf

The Gateway TypeScript types and service interfaces are automatically generated
from `pinpoint/proto/v1/gateway.proto`.

To re-generate the `pinpoint/webui/app/gateway/gateway.ts` definitions, run the
following command from the root of the `buildbot` repository:

```bash
BUILD_WORKSPACE_DIRECTORY=$PWD go generate ./pinpoint/proto/v1/...
```
