# Android Ingest Production Manual

## Running locally

Do a git checkout of the target repo:

    git clone https://skia.googlesource.com/git-master-skia

Then point to that directory as the url handed to androidingest as the
--repo_url. This will give you up to date commits, but also doesn't require
write access to the origin repo. You will probably have to be on a non-master
branch in the checkout so that the copy androidingest builds can push back to
it.

# Alerts

## process_failures

The process of creating git commits to mirror buildids has too
high on an error rate.

Check the logs for the exact operation in the process that is failing.

## tx_log

The storing of all uploaded data in the transaction log is failing. Check
GCS permissions and the logs for the errors generated.

## bad_files

Visit [android-metric-ingest.skia.org](https://android-metric-ingest.skia.org/)
and look at the "Recent Bad Requests" section and see why they are failing. A
previous issue has been bad serializing of the data that is POSTed to the server
where all the data was actually just encoded as one long string. If bad data is
being generated contact http://go/android-ingest-contact.

## liveness

The process of building the git repo is getting behind, check the logs to see
if it is the androidbuild API or Gerrit.

    $ kubectl logs -lapp=android_ingest | grep Timer:
