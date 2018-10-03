DESIGN
======


Overview
--------
Provides interactive dashboard for Skia performance data.

Code Locations
--------------

The code for the server along with VM instance setup scripts is kept in:

  * https://skia.googlesource.com/buildbot/+/master/perf/


Architecture
------------

This is the general flow of data for the Skia performance application.
The frontend is available at http://skiaperf.com.

```
                                         
               +-------------+             
               |             |             
               |   SKFE      |             
               |             |             
               |             |             
               |             |             
               +----------^--+             
                          |                
          +--------------------+----+-----+
          |   GCE Instance| skia-perf     |
          |               |               |
          |            ---+               |
          |            |                  |
          | +----------+-------------+    |
          | |        Perf (Go)       |    |
          | | ^    ^                 |    |
          | +------------------------+    |
          |   |    |                      |
          |   |    |                      |
          |   |    | +------------------+ |
          |   |    | |Tile Pipeline (Go)| |
          |   |    | |            ^     | |
          |   |    | +--+---------------+ |
          |   |    |    |         |       |
          +-------------------------------+
              |    |    |         |        
    +---------+-+  |    | +-------+--+     
    |   MySQL   |  |    | | Google   |     
    |           |  |    | | Storage  |     
    |           |  |    | |          |     
    |           |  |    | |          |     
    |           |  |    | |          |     
    |           |  |    | |          |     
    +-----------+  |    | +----------+     
                   |    |                  
                 +-+----v---+              
                 |   Tile   |              
                 |   Repo   |              
                 |          |              
                 |          |              
                 |          |              
                 |          |              
                 +----------+              
                                         
```

Perf is a Go application that serves the HTML, CSS, JS and the JSON representations
that the JS needs. It loads test results in the form of 'tiles' from the Tile Repo.
It combines that data with data about commits and annotations from the MySQL data base
and serves that the UI.

The Tile Pipeline is a separate application that periodically queries for fresh
data from Google Storage and then writes Tiles into the Tile Repo.

Tile Repo will be represented internally as an interface, the first
implemetation will be as files on the local disk, with a directory tree that
contains Go gob files called tiles.

Each tile contains exactly 128 points of every trace for a dataset.  The one
exception being the last tile, which may contain less that 128 points; see
below for an explanation of that.  The Tile Repo directory structure is:

    $TILE_REPO_ROOT/<dataset>/<scale>/<tilenumber>.gob

Where:

  * dataset = {skps|micro}
  * scale = 0..5 The scale factor of 4^N, so points in the /0/ directory
                 represent 1:1 with test results, while tiles in the /1/
                 directory have every fourth commit with data, and /2/
                 has every 128th commit with data.
  * tilenumber = The number of the tile, at the given scale, starting at BOT
                 (Beginning of Time).

When navigating the UI users can select the tiles they are looking at (<, >)
and also change the scaling factor that they are looking at (+,-).


URL Structure
-------------

The URL structure for retrieving Datasets is TBD.


Navigating
----------

For each point if the user wants to zoom out, add 1 to the scale factor and
divide tilenumber by two. Do the opposite to zoom in.  To move forwards or
backwards in time add or subtract 1 to the tile number. The actual UI
mechanisms for navigating around traces are TBD, this is just a description of
how the tiles are arranged.


Tile Pipeline Algorithm
-----------------------
TBD based on new ingestion code.


Logs
----

We use the https://github.com/golang/glog for logging, which puts Google style
Error, Warning and Info logs in /tmp/glog on the server under the 'perf'
account.


Debugging Tips
--------------

Starting the application is done via /etc/init.d/skiaperf which does the
backgrounding itself via start-stop-daemon, which means that if the app
crashes when first starting then nothing will make it to the logs. To debug
the cause in that case edit /etc/init.d/skiaperf and remove the --background
flag and then run:

  $ sudo /etc/init.d/skiaperf start

And you should get stdout and stderr output.

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

Monitoring of the application is done via Graphite at https://mon.skia.org.
Both system and application level metrics are monitored.


Annotations Database
--------------------

A Cloud SQL (a cloud version of MySQL) database is used to keep information on
Skia git revisions and their corresponding annotations. The database will be
updated when users add/edit/delete annotations via the dashboard UI.

MySQL Flags to set:

   max_allowed_packet = 1073741824

All passwords for MySQL are stored in valentine (search "skiaperf").

To connect to the database from authorized network (including skia-perf
GCE):

    $ mysql -h 173.194.104.24 -u root -p


    mysql> use skia

    mysql> show tables;

Initial setup of the database, the users, and the tables:

* Create the database and set up permissions. Execute the following after
  you connect to a MySQL database.

    CREATE DATABASE skia;
    USE skia;
    CREATE USER 'readonly'@'%' IDENTIFIED BY <password in valentine>;
    GRANT SELECT ON *.* TO 'readonly'@'%';
    CREATE USER 'readwrite'@'%' IDENTIFIED BY <password in valentine>;
    GRANT SELECT, DELETE, UPDATE, INSERT ON *.* TO 'readwrite'@'%';

* Create the versioned database tables.

  We use the 'perf_migratedb' tool to keep the database in a well defined (versioned)
  state. The db_host, db_port, db_user, and db_name flags allow you to specify
  the target database. By default it will try to connect to the production
  environment. But for testing a local MySQL database can be provided.

  Bring the production database to the latest schema version:

     $ perf_migratedb -logtostderr=true

  Bring a local database to the latest schema version:

     $ perf_migratedb -logtostderr=true -db_host=localhost --local


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

    foo:bar:baz:quux

Where the names come from the buildbot description, the test name, and the
configuration under which the test was run.

There are two other forms of trace ids:

  * Calculated traces - A trace that started out with a normal trace id, but
    then had a calculation performed on it. Calculated trace ids are not
    stored in shortcuts, but are presumed to be regenerated by any formula
    traces, which are stored in shortcuts.

    Calculated traces have IDs that begin with !. For example:

      !Arm7:Tegra2:Xoom:Android:ChunkAlloc_PushPop_640_480:nonrendering

  * Formula traces - A formula trace contains a formula to be evaluated which
    may generate either a single Formula trace that is added to the plot, such
    as ave(), or it may generate multiple calculated traces that are added to
    the plot, such as norm(). Note that formula traces are stored in shortcuts
    and added to plots even if it contains no data.

    Formula traces have IDs that begin with @. For example:

      @norm(filter("config=8888"))

        or

      @norm(filter("#54"))

Comparing bench results across verticals
----------------------------------------
The UIs showing line plots of selected traces and the clusterings are good ways
to examine and diagnose the performance of a small set of traces across the
commit timeline. However, it is not easy to use them for answering questions
like: "How does the performance of gpu config compare with 8888 on the SKP
benches across various platforms?", "What's the worst-performing SKPs on x86 vs.
x86_64, and are they worst on a specific OS?", and "How do I pinpoint the set of
potential performance changes introduced by a trybot run with my CL?". In this
case we care more about comparing the most recent data values by different
configs and platforms, instead of changes along the commit timeline.

To show overall comparison results across more dimensions, we use a table to
visualize the data. We use the same query interface for retrieving the set of
bench data of user's choice, but ask user to specify the search vertical (arch,
config, os, etc.) and select exactly two configs from it to compare against in
the query (say, "8888" and "gpu"). We then organize the data to calculate the
ratio of the benches from the two choices in the criteria (vertical) where all
other parameters are the same. For instance, we calculate the ratio of benches
in the "config" vertical from the following two traces:

    x86_64:HD7770:ShuttleA:Win8:gradient_create_opaque_640_480:gpu
    x86_64:HD7770:ShuttleA:Win8:gradient_create_opaque_640_480:8888

and put the value into the cell in a table that has
_row_gradient_create_opaque_640_480_ and
column _x86_64:HD7770:ShuttleA:Win8_. Basically, the table row will be the
"test" name, and the column will be the rest of the keys. The number of columns
will be the number of perf bots we run (20+ for now).

The value will then tell us if the performance is better (<1) or worse (>1) for
gpu against 8888. We can then heatmap-color the table cells by their value
ranges, to provide a visual way for users to identify the problems in cell
groups. By sorting the rows with aggregated performance, users will be able to
pinpoint the benches with worst/best relative performance to look into.

The same visulaization can be used for visualizing trybot results as well. When
user selects results from a recent trybot run (which is continually polled from
Google Storage as the ingester does, organized by try issue numbers and buildbot
/ build numbers), we pair the most recent bench results from regular buildbot
runs with the corresponding trybot bench results with identical trace keys, and
show their ratios in the table with heatmap-colored cells. The table row will
still be the "test" names, but the columns will concatenate all the other
verticals, such as _x86_64:HD7770:ShuttleA:Win8:gpu_. Users can control which
set of trybot results to show together with regular data, thus the number of
table columns is dynamic.

We can also add an option for users to specify a CL, so we use the available
bench data closest to that CL (either before or after) for visualization.

Another option is to have users provide two CLs and use the UI to show their
diffs on common traces.


Startup and config
------------------
Running skia perf is done via push. See ../push for more details.

You can always ssh into the server and start and stop the applications. The
server is started and stopped via:

    sudo /etc/init.d/skiaperf [start|stop|restart]

But sysv init only handles starting and stopping a program once, so we use
Monit to monitor the application and restart it if it crashes. The config
is in:

    /etc/monit/conf.d/perf

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


