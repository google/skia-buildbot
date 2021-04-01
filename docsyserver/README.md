# docsyserver

An application that serves up documentation stored in a git repository rendered
via Hugo using the Docsy template.

The application checks out the designated repository and renders all the
documentation that it then serves over HTTP.

It checks for updates to the repo every five minutes, pulling and re-rendering
the documentation.

In addition it understands the Gerrit code review system and allows for
previewing changes to documentation by visiting any URL into the documentation
with a `cl` query parameter, e.g. `?cl=NNNNN` and the documentation with that
reviews most recent changes to the documentation patched in and rendered.

The detailed design doc is at http://go/docsyserver.

## Directory Structure

- `images/release` - Builds a docker file that contains a checkout of the Docsy
  example repo, along with all the dependencies needed to build it, including
  Hugo and npm.

- `go/docsyserver` - The application.

- `go/codereview` - An abstraction of the functionality used from Gerrit. In
  theory a new implementation of CodeReview could be made that supports GitHub
  or other code review systems.

- `go/docsy` - An abstraction of running the `hugo` executable over the source
  documentation using the Docsy template.

- `go/docset` - Manages checking out the repo, patching CLs for documentation
  under code review, and cleaning up after code review issues are closed.

- `images/head-end.htm` - If there are script sources that need to be added to
  each page those should go in this file, which will be placed in the right
  directory in the docsy template.

## Building

To build a local docker image run:

        $ make

This will build the executable and store it at `$GOPATH/bin/docsyserver`.

## Running locally with docker

First build the docker image, which only needs to be done once:

      $ make build-local-image

Then run a local instance, changing `$(SKIA)` to the local checkout of Skia:

      docker run --entrypoint=/serve.sh -ti -p 1313:1313 -v $(SKIA)/site:/input docsyserver:latest

The content will now be available at [localhost:1313](http://localhost:1313/).
The server will automatically re-render the HTML and refresh the page as the
source documents are edited, there's no need to restart the server after you
make documentation changes.

## Running locally

To run, execute the docker image and supply the following flags:

        $ docsyserver --local [flags]

```
  -alsologtostderr
        log to standard error as well as files
  -doc_path string
        The relative directory, from the top of the repo, where the documents are located. (default "site")
  -doc_repo string
        The repo to check out. (default "https://skia.googlesource.com/skia")
  -gerrit_url string
        The gerrit URL. (default "https://skia-review.googlesource.com")
  -hugo string
        The absolute path to the hugo executable. (default "hugo")
  -port string
        HTTP service address (e.g., ':8000') (default ":8000")
```

You must have the `gcloud` command line tool installed and authorized, as that's
how docsyserver with the `--local` flag will create an OAuth 2.0 bearer token to
access Gerrit. You will also need a local checkout of the Docsy example project
and Hugo installed. See the
[Docsy docs](https://www.docsy.dev/docs/getting-started/) for installation
instructions.

The full set of flags are:

```
  -alsologtostderr
        log to standard error as well as files
  -doc_path string
        The relative directory, from the top of the repo, where the documents are located. (default "site")
  -doc_repo string
        The repo to check out. (default "https://skia.googlesource.com/skia")
  -docsy_dir string
        The directory where docsy is found. (default "../../docsy-example")
  -gerrit_url string
        The gerrit URL. (default "https://skia-review.googlesource.com")
  -hugo string
        The absolute path to the hugo executable. (default "hugo")
  -local
        Running locally if true. As opposed to in production.
  -log_backtrace_at value
        when logging hits line file:N, emit a stack trace
  -log_dir string
        If non-empty, write log files in this directory
  -logtostderr
        log to standard error instead of files
  -port string
        HTTP service address (e.g., ':8000') (default ":8000")
  -prom_port string
        Metrics service address (e.g., ':10110') (default ":20000")
  -stderrthreshold value
        logs at or above this threshold go to stderr
  -v value
        log level for V logs
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging
  -work_dir string
        The directory to check out the doc repo into. (default "/tmp")
```
