## Usage

Run `backfill <command>`! You can `backfill help git` for more detailed
docs for each command.

### git

The `git` subcommand reads output from git log and writes the results to the
Revision entity group in datastore, including generation numbers. The git log
should be generated from running:
`git log --topo-order --reverse -z --format=format:'%H,%P,%ae,%ct,%b'`

This should be done from a clone without local edits.

## Enabling remote API
You need to deploy some module to app engine which has the remote api enabled
in order for this backfiller to work successfully.
Full docs here: https://cloud.google.com/appengine/docs/go/tools/remoteapi

You *should* be able to just `goapp deploy` in `/milo/appengine/remote_api`,
and then do `backfill buildbot -remoteURL
remote-api-1-dot-luci-milo.appspot.com`, or something like that.
