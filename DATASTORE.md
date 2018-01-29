Google Cloud Datastore
======================

We use the Namespace in Cloud Datastore to differentiate applications (Perf,
Gold) and different installations of the same application (Perf, Perf-Android,
Gold, Gold-PDFium). Since there may end up being many namespaces, a naming
convention has been established:

Naming convention for namespaces
--------------------------------

    <app>[-<instance>]
    <app>-localhost-<user>

Where 'instance' is the name of the instance, or blank if there is a canonical
instance.

Examples:

    perf
    perf-androidmaster
    perf-localhost-jcgregorio
    gold
    gold-pdfium
    gold-localhost-stephana

Indexes
-------

Composite Indexes for all applications are defined in `ds/index.yaml`. Keeping
the indexes in a single location makes it possible to run
[gcloud datastore cleanup-indexes](https://cloud.google.com/sdk/gcloud/reference/datastore/cleanup-indexes).
