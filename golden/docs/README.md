Gold Manual for Clients
=======================

What is Gold?
=============

Gold is used in automated tests to answer "Did my code draw the right thing?"
It has the following major features:
 - Multiple correct (or "positive") images for a single test.
 - Pre-submit validation can cause tests to fail if output is correct. (opt-in)
 - Post-submit validation can let a commit land first and alert humans if something
   drew incorrectly after the fact (useful when automated tests are slow/busy).
 - TryJob support to see any images that would be produced by a commit and then triage them
   (i.e. mark as positive/negative) before they land.
 - Web UI to show incorrect images and various other analytics.
   [This page](https://skia.org/dev/testing/skiagold) has some older, but still representative
   pictures of the UI pages.
 - Scales to over 1 million images per commit.

Using Gold
==========

There are two parts to Gold, the "server part" and the "test part". The "server part", also known as
the "Gold instance", runs on a web server and ingests the data from an executable run alongside the
tests.

Setting up the Gold Instance
----------------------------

**If you are part of a Google team**, the Skia infra team can host your instance for you.
[File a bug](https://bugs.chromium.org/p/skia/issues/entry?template=New+Gold+Instance) and we'll
get back to you.

Otherwise, you'll need approximately the following steps. We use Google Cloud and Kubernetes (k8s)
and the following assumes you do too.

 1. Create a Google Cloud Project - this will house the data, credentials, configuration, and
    k8s pods that make up Gold.
 2. Make a Google Cloud Storage (GCS) bucket in the project. This specifically will be the source
    of truth for Gold. All data uploaded from tests will live here and be interpreted by Gold.
 3. [deprecated] Make a BigTable Table - this will house some data being processed by Gold.
 4. Create a CockroachDB cluster. This is the new way to store all data in Gold.
 5. Make a service account that can write to the GCS bucket. This will be how you authenticate to
    goldctl (see below).
 6. Create a k8s deployment of ingestion. This will read the data out of GCS and put it into
    BigTable (or Firestore for TryJobs). All our Dockerfiles are in ../dockerfiles
    and the templates we use for deployments are in ../k8s-config-templates.
 7. Create a k8s deployment of diffcalculator. This will compute the differences between images
    and output things like the diff metrics and images visualizing the differences. This will
    need a PubSub topic/subscription created (see cmd/pubsubtool).
 8. Create a k8s deployment of frontend.
 9. Create a k8s deployment of baselineserver. This is a lighter-weight and more highly-available
    subset of the frontend, which will be queried by goldctl.

Integrating with your tests
---------------------------

The primary way to integrate with your tests is to use `goldctl` (pronounced "gold-control"),
a helper binary that speaks to the Gold Frontend and uploads data to the GCS bucket.

To install `goldctl` from source, make sure you a have a recent version of Go installed.
Then it can be installed with:

```console
   $ go get -u go.skia.org/infra/gold-client/cmd/goldctl
```

**If you are part of a Google team that uses CIPD**, goldctl is available via
[CIPD](https://chrome-infra-packages.appspot.com/p/skia/tools/goldctl).

A typical workflow for using Gold in a post-submit fashion is:

```console
    # Set up authentication (Google folks, there's also the --luci option, which may be useful)
    goldctl auth --work-dir ./tmp --service-account /path/to/serviceaccount.json

    # The keys.json is a JSON file that has key-value pairs describing how these inputs
    # got drawn. Commonly, you might specify the OS, the GPU, etc. For instances owned by the
    # Skia team, the --instance is a nice shortcut (assigned when the instance is created),
    # For other clients, goldctl has a --url to specify the webserver endpoint.
    goldctl imgtest init --work-dir ./tmp --keys-file ./keys.json --instance my-instance

    # Run a test named "cute-dog", assume it outputs /out/foo.png

    # This command will upload the image to the bucket and store an entry for test-name linking
    # to that image.
    goldctl imgtest add --work-dir ./tmp --test-name "cute-dog" --png-file /out/foo.png

    # Run many more tests

    # Upload a JSON file to the GCS bucket that includes all the test entries from this run.
    goldctl imgtest finalize --work-dir ./tmp
```

Using Gold in pass/fail mode (i.e. useful for presubmits), looks very similar but has a few
small differences:
 - add `--passfail` (and optionally `--failure-file`) to the `goldctl imgtest init` call.
 - Omit the `goldctl imgtest finalize` call at the end. In pass/fail mode, individual JSON files
   are uploaded in a "streaming" fashion instead of one big file at the end.

Of note, the `goldctl imgtest init` call is optional; it just makes the future calls less verbose
by specifying things once instead of multiple times.

For more, try adding `--help` to the various `goldctl` commands.