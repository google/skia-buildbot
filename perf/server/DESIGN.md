DESIGN
======


Overview
--------
Provides interactive dashboard for Skia performance data.

Code Locations
--------------

The code for the server along with VM instance setup scripts is kept in:

  * https://skia.googlesource.com/buildbot/+/master/perf/server


Architecture
------------

This is the general flow of data for the Skia performance application.
The frontend is available at http://skiaperf.com.

                                         
               +-------------+             
               |             |             
               |   Browser   |             
               |             |             
               |             |             
               |             |             
               +----------^--+             
                          |                
          +--------------------+----+-----+
          |   GCE Instance|skia+perf+b    |
          |               |               |
          |   +-----------+----------+    |
          |   |     Squid3           |    |
          |   |                      |    |
          |   +--------^-------------+    |
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
    |   MySQL   |  |    | | BigQuery |     
    |           |  |    | |          |     
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
                                         
Perf is a Go application that serves the HTML, CSS, JS and the JSON representations
that the JS needs. It loads test results in the form of 'tiles' from the Tile Repo.
It combines that data with data about commits and annotations from the MySQL data base
and serves that the UI.

The Tile Pipeline is a separate application that periodically queries for fresh
data from BigQuery and then writes Tiles into the Tile Repo. Note that when
ingestion moves out of prod and into the same server we can do the tile updates
immediately after ingestion is done.

Tile Repo will be represented internally as an interface, the first
implemetation will be as files on the local disk, with a directory tree that
contains gzipped JSON files called tiles. Note that we may alternatively use Go
native gob encoded files and just transform then into JSON when serving them to
the UI.

Each tile contains exactly 16 points of every trace for a dataset.  The one
exception being the last.gz tile, which may contain less that 16 points; see
below for an explanation of that.  The Tile Repo directory structure is:

    $TILE_REPO_ROOT/<dataset>/<scale>/<tilenumber>.gz

Where:

  * dataset = {skps|micro}
  * scale = 0..5 The scale factor of 4^N, so points in the /0/ directory
                 represent 1:1 with test results, while tiles in the /1/
                 directory have every fourth commit with data, and /2/
                 has every 16th commit with data.
  * tilenumber = The number of the tile, at the given scale, starting at BOT
                 (Beginning of Time).

So the data in:

    /skps/0/0.gz

contains the data for the first 16 commits from the BOT that have test data.


    /micro/3/6.gz

contains the 16 commits per trace of every 64th commit that has data and is the
7th data set in that order. So it contains 16 points per trace, and each point
falls between 6 * 64 * 16=6144 and 7 * 64 * 16=7168, i.e it is a slice of 16
commits that represent a range of 1024 commits.

A manifest file will be available that give the commit timestamp ranges for
each tile.

When navigating the UI users can select the tiles they are looking at (<, >)
and also change the scaling factor that they are looking at (+,-).

Each /dataset/scale directory also contains a file, last.gz, that contains the
most recent data, from 1 to 16 points. The Tile Pipeline process will update
the 'last.gz' files in each directory and write new tile files as new data
arrives. Last.gz will contain the id of the tile that appears just before it,
that way the UI can request /skps/0/last.gz and know how to proceed from
there.

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
Start at /0/last.gz and find the previous tilenumber.gz, open that and find
the last githash. Query for all data newer than that githash.  Group into tiles
of 16 githashes. Put the remainder (or the last 16 if the remainder is 0) into
/0/last.gz.

Now do the same for /1/, but the data comes from /0/ and not from BigQuery.
I.e.  /1/0.gz is just a sampling of /0/0.gz, /0/1.gz, /0/2.gz and /0/3.gz.
At each level only rewrite last.gz and write out any new complete tiles as
they are filled.


Perf Stats Database
-------------------

The data for the performance metrics are kept in the BigQuery tables stored
in the google.com:chrome-skia project. Note that this is a different project
from where the data is accessed, which is by VM instances running under
the google.com:skia-buildbots project. For this to work the service account
email of the VM needs to be added to the permissions group of the
google.com:chrome-skia project. If this isn't done then the BigQuery access
will fail with a 403 error.


Logs
----

We use the https://github.com/golang/glog for logging, which puts Google style
Error, Warning and Info logs in /tmp on the server under the 'perf' account.


Debugging Tips
--------------

Starting the application is done via /etc/init.d/perf which does the
backgrounding itself via start-stop-daemon, which means that if the app
crashes when first starting then nothing will make it to the logs. To debug
the cause in that case edit /etc/init.d/perf and remove the --background
flag and then run:

  $ sudo /etc/init.d/perf start

And you should get stdout and stderr output.

Monitoring
----------

Monitoring of the application is done via Graphite at http://skiamonitor.com.
Both system and application level metrics are monitored.


Annotations Database
--------------------

A Cloud SQL (a cloud version of MySQL) database is used to keep information on
Skia git revisions and their corresponding annotations. The database will be
updated when users add/edit/delete annotations via the dashboard UI.

All passwords for MySQL are stored in valentine (search "skia perf").

To connect to the database from authorized network (including skia-perf GCE):

    $ mysql -h 173.194.104.24 -u root -p

Initial setup of the database, the users, and the tables:

    CREATE DATABASE skia;
    USE skia;
    CREATE USER 'readonly'@'%' IDENTIFIED BY <password in valentine>;
    GRANT SELECT ON *.* TO 'readonly'@'%';
    CREATE USER 'readwrite'@'%' IDENTIFIED BY <password in valentine>;
    GRANT SELECT, DELETE, UPDATE, INSERT ON *.* TO 'readwrite'@'%';

    // Table for storing annotations.
    CREATE TABLE notes (
      id     INT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
      type   TINYINT,
      author TEXT,
      notes  TEXT      NOT NULL
    );

    // Table for storing git revision information.
    CREATE TABLE githash (
      githash   VARCHAR(40)   NOT NULL PRIMARY KEY,
      ts        TIMESTAMP     NOT NULL,
      gitnumber INT           NOT NULL,
      author    TEXT          NOT NULL,
      message   TEXT          NOT NULL
    );

    // Table for mapping revisions and annotations. This support many-to-many
    // mapping.
    CREATE TABLE githashnotes (
      githash VARCHAR(40)  NOT NULL,
      ts      TIMESTAMP    NOT NULL,
      id      INT          NOT NULL,

      FOREIGN KEY (githash) REFERENCES githash(githash),
      FOREIGN KEY (id) REFERENCES notes(id)
    );

Common queries that the dashboard will use:

    INSERT INTO notes (type, author, notes) VALUES (1, 'bsalomon', 'Alert!');

    SELECT LAST_INSERT_ID();

    INSERT INTO githashnotes (ts, id) VALUES (<githash_ts>, <last_insert_id>);

The above set of commands will usually be used together to add new annotations
and associate them with corresponding git commits. The commands below remove an
annotation and its associations with any commit.

    DELETE FROM githashnotes WHERE id = <id_to_delete>;

    DELETE FROM notes WHERE id = <id_to_delete>;

Since the data size is relatively small, the dashboard server can keep a copy of
all recent commit info (e.g., for constructing a "blamelist"), annotations, and
their many-to-many relationship for use in the context.

Password for the database will be stored in the metadata instance. To see the
current password stored in metadata and the fingerprint:

    gcutil --project=google.com:skia-buildbots getinstance [skia-perf GCE instance]

To set the mysql password that perf is to use:

    gcutil --project=google.com:skia-buildbots setinstancemetadata [skia-perf GCE instance] --metadata=readonly:[password-from-valentine] --metadata=readwrite:[password-from-valentine] --fingerprint=[the metadata fingerprint]


Startup and config
------------------
The server is started and stopped via:

    sudo /etc/init.d/perf [start|stop|restart]

But sysv init only handles starting and stopping a program once, so we use
Monit to monitor the application and restart it if it crashes. The config
is in:

    /etc/monit/conf.d/perf

Installation
------------
See the README file.
