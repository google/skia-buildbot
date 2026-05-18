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
