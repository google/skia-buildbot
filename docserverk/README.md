# docserver

A super simple Markdown (CommonMark) server in Go.

This application serves up processed Markdown files that are
stored in a Git repository. It allows push-to-deploy for updating
Markdown files, i.e. every 15 minutes the server pulls the Markdown
repo to head.

The application also supports previewing CLs against the Markdown
repository. Just append `?cl={Reitveld_issue_number}` to any
URL and you can see what the file will render with the changes
from that CL.

## Design

The docserver presumes the Git repository that contains the Markdown files will
exists under the /site directory in the repo. Each directory should have an
`index.md` file which is the content for that directory, and any number of
`.md` files beyond the `index.md` file which are the contents of a directory.
Other assetts may appear in the directory structure and will be served over
HTTP as files. See below for an example directory layout:

```
  site
  ├── dev
  │   ├── contrib
  │   │   ├── codereviews.md
  │   │   └── index.md
  │   └── index.md
  ├── index.md
  ├── logo.png
  ├── roles.md
  ├── roles.png
  ├── user
  │   ├── api.md
  │   ├── download.md
  │   ├── index.md
  │   ├── issue-tracker.md
  │   └── quick
  │       ├── index.md
  │       └── linux.md
  └── xtra
      └── index.md
```

The server will build a navigation menu for the site by walking the directory
structure in alphabetical order and will use each Markdown documents first line
as the title of the document in the navigation bar.

## Running locally

Requirements:

1. [Go](https://golang.org)

Clone this repo:

    git clone https://skia.googlesource.com/buildbot

Then build the executable:

    cd buildbot/docserverk

    make

Now run docserverk from within the docserverk directory. If $GOPATH/bin is in
your $PATH then you can run:

    docserverk  --logtostderr --local

otherwise run:

    $GOPATH/bin/docserverk  --logtostderr --local

The server will eventually log "Ready to serve" at which point you can visit

    http://localhost:8000
