Push
====

A set of applications and conventions for making it easy to push new versions
of applications to servers we manage. Key design targets are easy deploys and
easy rollbacks with minimal disruption of our current infrastructure.

There are several components to the system, as seen below:

```

  +------------------------+
  |                        |
  | Build Release Package  |
  |                        |
  +-----------+------------+
              |
              |
              v
  +-----------------------+      +-------------------+
  |                       |      |                   |
  | Google Storage        |      |   Push Server     |
  |   gs://skia-push/     | <--> |                   |
  |                       |      |                   |
  |                       |      |                   |
  +-------------+---------+      +-------------------+
                |
                |
                v
     +-------------------+
     |                   |
     | +-----------------+-+
     | |                   |
     | |  +----------------+--+
     | |  |                   |
     | |  | GCE Instance      |
     +-+  |   running         |
       +--+   pulld client    |
          |                   |
          +-------------------+

```

Build Release Package
---------------------

The process begins with building a Debian package that contains all the assets
needed, including a monit config file and a SysV style init file if you are
deploying an application. Note that not only applications can be deployed this
way, for example nginx config files could be deployed by this system also.

The script in `../bash/release.sh` builds the debian package and uploads it to
the correct spot in Google Storage.


Google Storage
--------------

There is a bucket solely for managing push images and configuration and it has
the following top level structure:

    gs://skia-push
    gs://skia-push/debs/
    gs://skia-push/server/

Under /debs/ the Debian images produced from the above step are stored. They
are stored in this manner:

    gs://skia-push/debs/{application name}/{uniquely named Debian package}.deb

Under `/server/` there will be a single JSON file for each server that
describes the packages that should be installed on that server. The package
list is of the actual Debian image names held in `/debs/`.

    gs://skia-push/server/{server name}.json

For example, `gs://skia-push/server/skia-push.json` looks like this:

    [
      "pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-09T19:05:02Z:7e0ff6059e653eb75a36efb621d0fd66d1ced433.deb",
      "push/push:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-09T18:45:59Z:2d9eea8b121f61ac8f1ab853fedc30fc092b3f70.deb",
      "logserver/logserver:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-09T18:15:56Z:52a8ec0da66d45a58274947080870b742404a92f.deb"
    ]


Pull Client
-----------

On every GCE instance that is managed by the Push Server there is a long
running 'pulld' process that polls the
`gs://skia-push/server/{servername}.json` file and looks for it to change.
When it does change then any new Debian packages are downloaded from
`gs://skia-push/debs` and installed.
The push server will also send a request to
http://[server-name]:10116/pullpullpull to trigger pulld to look for changes
in the json file.


GCE Instances
-------------

GCE instances that are created with a boot disk built from the
'skia-pushable-base' snapshot are fully ready to be push clients.  New packages
that are part of Debian can't be installed by the push process, for example,
nginx. Installing such packages on an instance is done by using a [startup
script](https://cloud.google.com/compute/docs/startupscript).

Similarly mounting other drives should be done via startup script.

Push Server
-----------

A web application that makes it easy to choose from the existing releases of
each package and choose them to be deployed to each server. It also has one
config file that determines which applications are allowed to be deployed in
which servers.

    skiapush.conf

The server reads all the information from `gs://skia-push` to build its UI,
and on user selection of application versions to deploy it will read and write
back a modified file to `gs://skia-push/servers/{servername}.json`. That will
trigger the selected server to update that package during the next polling
cycle (currently every 15 seconds).

