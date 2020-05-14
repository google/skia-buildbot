DESIGN
======

Overview
--------
Provides interactive dashboard for Skia performance data.

Code Locations
--------------

The code for the server along with VM instance setup scripts is kept in:

  * https://skia.googlesource.com/buildbot/+show/master/perf/


Architecture
------------

This is the general flow of data for the Skia performance application.
The frontend is available at http://perf.skia.org for Skia.

```
               +-------------+
               |             |
               |   Ingress   |
               |             |
               |             |
               |             |
               +-------------+
                          ^
                          |
              GKE Instance| skia-perf
                          |
                       ---+
                       |
            +----------+-------------+
            |        Perf (Go)       |
            +------------------------+
              ^    ^       ^
              |    |       | (PubSub Events)
              |    |       |
              |    | +-----+--------------+
              |    | | Perf Ingester (Go) |
              |    | +--+-----------------+
              |    |    |       ^
              |    |    |       |
              v    |    |       |
    +---------+-+  |    | +-----+----+
    | Datastore |  |    | | Google   |
    |           |  |    | | Storage  |
    +-----------+  |    | +----------+
                   |    v
                 +-+--------+
                 |   Tile   |
                 |   Store  |
                 +----------+

```

Perf is a Go application that serves the HTML, CSS, JS and the JSON representations
that the JS needs. It loads test results in the form of 'tiles' from the Tile Store.
It combines that data with data about commits and annotations from Google Datastore
and serves that the UI.

The Perf Ingester is a separate application that periodically queries for fresh
data from Google Storage and then writes Traces into the Tile Store. It
generates PubSub events for each file it ingests. The Tile Store is currently
implemented on top of Google BigTable.

Users
-----

Users must be logged in to access some content or to make some changes in the
application, such as changing the status of perf alerts. User authentication
is handled through OAuth 2.0, in this case specifically tied to the Google
implementation. Once the OAuth 2.0 permission grant is complete then the users
email is used as an identifer. The authentication is not stored on the server,
instead it is stored as a cookie in the browser and verified when
authentication is needed.

There are two APIs, one in Go and another in Javascript that are used to
access the current user and their logged in status:

In Go the login.LoggedInAs(), see go/login/login.go.

In Javascript the interface is sk.Login which is a Promise, see
res/imp/login.html.

Monitoring
----------

Monitoring of the application is done via Graphite at https://grafana2.skia.org.
Both system and application level metrics are monitored.


Clustering
----------

The clustering is done by using k-means clustering over normalized Traces. The
Traces are normalized by filling in missing data points so that there is a
data point for every commit, and then scaling the data to have a mean of 0.0
and a standard deviation of 1.0. See the docs for ctrace.NewFullTrace().

The distance metric used is Euclidean distance between the traces.

After clustering is complete we calculate some metrics for each cluster by
curve fitting a step function to the centroid. We record the location of the
step, the size of the step, and the least squares error of the curve fit. From
that data we calculate the "Regression" value, which measures how much like a
step function the centroid is, and is calculated by:

  Regression = StepSize / LeastSquaresError.


The better the fit the larger the Regression, because LSE gets smaller
with a better fit. The higher the Step Size the larger the Regression.

A cluster is considered "Interesting" if the Regression value is large enough.
The current cutoff for Interestingness is:

  |Regression| > 150

Where negative Regression values mean possible regressions, and positive
values mean possible performance improvement.

Alerting
--------

A dashboard is needed to report clusters that look "Interesting", i.e. could
either be performance regressions, improvements, or other anomalies. The
current k-means clustering and calculating the Regression statistic for each
cluster does a good job of indicating when something Interesting has happened,
but a more structured system is needed that:

  * Runs the clustering on a periodic basis.
  * Allows flagging of interesting clusters as either ignorable or a bug.
  * Finds clusters that are the same from run to run.

The last step, finding clusters that are the same, will be done by
fingerprinting, i.e. use the first 20 traces of each cluster will be used as a
fingerprint for a cluster. That is, if a new cluster has some (or even one) of
the same traces as the first 20 traces in an existing cluster, then they are
the same cluster. Note that we use the first 20 because traces are stored
sorted on how close they are to the centroid for the cluster.

Algorithm:
  Run clustering and pick out the "Interesting" clusters.
  Compare all the Interestin clusters to all the existing relevant clusters,
    where "relevant" clusters are ones whose Hash/timestamp of the step
    exists in the current tile.
  Start with an empty "list".
  For each cluster:
    For each relevant existing cluster:
      Take the top 20 keys from the existing cluster and count how many appear
      in the cluster.
    If there are no matches then this is a new cluster, add it to the "list".
    If there are matches, possibly to multiple existing clusters, find the
    existing cluster with the most matches.
      Take the better of the two clusters (old/new) based on the better
      Regression score, i.e. larger |Regression|, and update that in the "list".
  Save all the clusters in the "list" back to the db.

This algorithm should keep already triaged clusters in their triaged
state while adding new unique clusters as they appear.

Example
~~~~~~~

Let's say we have three existing clusters with the following trace ids:

    C[1], C[2], C[3,4]

And we run clustering and get the followin four new clusters:

    N[1], N[3], N[4], N[5]

In the end we should end up with the following clusters:

    C[1] or N[1]
    C[2]
    C[3,4] or N[3] or N[4]
    N[5]

Where the 'or' chooses the cluster with the higher |Regression| value.

Each unique cluster that's found will be stored in the datastore. The schema
will be:

    CREATE TABLE clusters (
      id         INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
      ts         TIMESTAMP    NOT NULL,
      hash       TEXT         NOT NULL,
      regression FLOAT        NOT NULL,
      cluster    MEDIUMTEXT   NOT NULL,
      status     TEXT         NOT NULL,
      message    TEXT         NOT NULL
    );

Where:
  'cluster' is the JSON serialized ClusterSummary struct.
  'ts' is the timestamp of the step in the step function.
  'status' is "New" for a new cluster, "Ignore", or "Bug".
  'hash' is the git hash at the step point.
  'message' is either a note on why this cluster is ignored, or a bug #.

Note that only the id may remain stable over time. If a new cluster is found
that matches the fingerprint of an exisiting cluster, but has a higher
regression value, than the new cluster values will be written into the
'clusters' table, including the ts, hash, and regression values.

~~~~~~~

Trace IDs
---------

Normal Trace IDs are of the form:

    ,key=value,key2=value2,

See go/query for more details on structured keys.

There are two other forms of trace ids:

  * Formula traces - A formula trace contains a formula to be evaluated which
    may generate either a single Formula trace that is added to the plot, such
    as ave(), or it may generate multiple calculated traces that are added to
    the plot, such as norm(). Note that formula traces are stored in shortcuts
    and added to plots even if it contains no data.

    Formula traces have IDs that begin with @. For example:

      norm(filter("config=8888"))

        or

      norm(filter("#54"))

Installation
------------
See the README file.

Ingestion
---------

Ingestion is now event driven, using PubSub events from GCS as files
are written. The naming convention for those PubSub topics is:

    <app name>-<function>-<instance>

For example, for Perf ingestion of Skia data the topic will be:

    perf-ingestion-skia


Event Driven Alerting
---------------------

Instead of running continuously over all Alert configs and running the
regressions found there it may be beneficial in some cases to look for
regressions only when new data has arrived.

The current system for Alerting was built on the assumption of smaller data sets
with dense data arriving at a steady rate, i.e. kicking off Alerting once an
hour and having it finish in much less than an hour was expected.

With the arrival of Android's data which is:

   1. Sparse
   2. Combinatorially much larger than Skia's data set, 40M traces as compared to Skia's 400K.
   3. Alerts that need to be grouped along both Branch and BuildFlavor.

Because of this the continuous clustering process for Android is now consuming a
huge number of cores (60 GCE cores and 10 BT cores) and BT bandwidth (15M
rows/s) and also has high latency (on the order of 12-24 hours), even after the
indexing system has been rebuilt.

To solve this problem Alerting should optionally be done on an incremental
process, that is, as data arrives, i.e. as it passes through the ingesters, a
PubSub event will be generated which will include the names of all the traces
that have just been updated.

PubSub has a limit of 10MB for data in a single event, so we can send a gzipped
list of trace ids as the body, the trace ids are highly redundant and should
compress down to a very small size. A spot check of Skia JSON files shows them
to be about 300K zipped and since the values are being dropped the zipped size
should be even smaller.

As each PubSub event arrives at a clusterer it will be checked against each
Alert to see if the new traces match the Alert, if so then the Alert will be
run, but for only the Group By configs that match the paramset that represents
all the traceids, a savings of up to 1000x for Android alerts today. This will
also dramatically reduce the latency of Alerts sent down from 1 day to less than
a minute.

Note that this system will not work for CT where data arrives in batches of 1M
trace ids, nor will it be a savings for dense data sets like Skia, so those
instances should stick with the existing system that clusters continuously over
all Alerts.