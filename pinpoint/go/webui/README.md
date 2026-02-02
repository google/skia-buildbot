# Pinpoint Web UI

This directory contains the Go backend for the Pinpoint Web UI.

## Running Locally

To build and run the Go server directly without Docker:

```bash
bazelisk run //pinpoint/go/webui -- --port=:8000
```

## Local Deployment with Docker

### 1. Build and Load the Container

The `skia_app_container` macro automatically generates a `load_<name>` target
that builds the OCI image and loads it into your local Docker daemon.

Run the following command:

```bash
bazelisk run //pinpoint:load_pinpoint_webui
```

### 2. Run the Container

Once loaded, you can run the container using standard Docker commands. The image
is tagged as `gcr.io/` followed by the `repository` attribute defined in your
`BUILD.bazel`.

```bash
docker run -p 8000:8000 gcr.io/skia-public/pinpoint_webui:latest
```

### 3. Verify

Access the service in your browser at:
[http://localhost:8000](http://localhost:8000)

## 401 Unauthorized Error

If you see an error like `GET returned 401 Unauthorized` when fetching
`@base-cipd` (or any other `gcr.io` image), it is an authentication issue.

### Fix: Re-authenticate and Configure Docker

Run the following commands in your terminal:

1.  **Login to gcloud**: `bash gcloud auth login`
2.  **Set up Application Default Credentials (ADC)**: `bash gcloud auth
application-default login`
3.  **Configure Docker to use gcloud as a credential helper**: Bazel and
    `rules_oci` often rely on your Docker configuration to authenticate with
    GCR. `bash gcloud auth configure-docker gcr.io`
