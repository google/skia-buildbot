# Competitive Performance UI

Run popular browser benchmarks on a range of browsers and store the results in a
Perf instance.

[Design Doc](http://go/comp-ui)

## Deployment

The comp-ui-cron-job needs to be built and deployed via an Ansible script since
it runs in the skolo and contains an embedded service account key that allows it
to upload the run results to the Google Cloud Storage bucket.
