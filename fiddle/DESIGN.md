Fiddle 2.0 Design
=================

Requirements
------------

1. URLs must not break from the old version of fiddle to the new version of
   fiddle.
1. Must be able to build and run the same fiddle at different versions of
   Skia, for example, HEAD, m50, and m49. (Where HEAD actually means last roll
   of Skia into Blink.)
1. Need to be able to run a local version for testing, even if the local
   system is not systemd based.
1. Must use our latest techniques and tools, i.e. InfluxDB metrics, push, and
   logging.
1. Polymer 1.0 only.
1. Switch to systemd-nspawn from chroot.
1. Drop support for workspaces. They never appeared in humper's UI and they
   appear to have been broken for at least the last 6 months, if not longer,
   and no one noticed.
1. Add an admin UI that allows deleting uploaded images, including the
   generated images from fiddles run against that image.

Building
--------

There are several executables that need to get built at various times
and places that go into compiling and running user's untrusted C++ code.
The following executables part of that process:

  * systemd-nspawn - Part of systemd, launches a chroot jail.
  * fiddle\_secwrap - Runs a program under the control of ptrace.
  * fiddle\_main - The user's code with a wrapper to load a source
                  image and to write out the resulting PNGs, PDF,
                  an SKP data.
  * fiddle\_run - This is run within the chroot jail. It compiles
                 the user's code against fiddle\_main.o and
                 libskia.so and then runs the resulting executable
                 under the control of fiddle\_secwrap. It gathers
                 the output of all steps and emits it as one large
                 JSON file written to stdout.


Some of these are built and included as part of the push package:

~~~~
    build_release
    +---------------------------------------------+
    |                                             |
    |  fiddle_secwrap.cpp +----> fiddle_secwrap   |
    |  fiddle_run/main.go +----> fiddle_run       |
    |                                             |
    +---------------------------------------------+
~~~~

The rest are built on the server as it runs:

~~~~
    skia-fiddle
    +----------------------------------------------------------+
    |                                                          |
    |                                    cmake                 |
    |  FIDDLE_ROOT/versions/<githash>/  +-----> libskia.so     |
    |  FIDDLE_ROOT/.../fiddle_main.cpp  +-----> fiddle_main.o  |
    |                                                          |
    |                                                          |
    |                                                          |
    |  User's code written and mounted in container at:        |
    |    FIDDLE_ROOT/src/draw.cpp                              |
    |                                                          |
    |                                                          |
    |                                                          |
    |  systemd-nspawn                                          |
    |    +                                                     |
    |    |                                                     |
    |    +-> fiddle_run (stdout produces JSON)                 |
    |           +       (capture stdout/stderr of child procs) |
    |           |                                              |
    |           |   draw.cpp        cmake                      |
    |           +-> fiddle_main.o  +----->  fiddle_main        |
    |           |   libskia.so                                 |
    |           |                                              |
    |           |                                              |
    |           +-> fiddle_secwrap                             |
    |                   +                                      |
    |                   |                                      |
    |                   +-> fiddle_main                        |
    |                                                          |
    |                                                          |
    +----------------------------------------------------------+
~~~~


By default $FIDDLE\_ROOT is /mnt/pd0, but can be another directory when running
locally and not using systemd-nspawn.

Skia is checked out into $FIDDLE\_ROOT/versions/<githash>, and cmake built,
with the output going into $FIDDLE\_ROOT/versions/<githash>/cmakeout.
Good builds are recorded in $FIDDLE\_ROOT/goodbuilds.txt, which is just
a text file of good builds in the order they are done, that is, new
good builds are appended to the end of the file.

The rest of the work, compiling the user's code and then running it, is done
in the container, i.e. run in a root jail using systemd-nspawn.

In the container, / is mounted read-only. Also bind a directory
$FIDDLE\_ROOT/src/ as read-only, where the source for $FIDDLE\_ROOT/src/ is
$FIDDLE\_ROOT/tmp/<tmpdir>/, where tmpdir is unique for each requested compile.
(This is just a symbolic link when not running via nspawn.) Also mount
/tmp/<tmpdir> as read/write at $FIDDLE\_ROOT/out for the executable output of
the compile.  Compile $FIDDLE\_ROOT/src/draw.cpp and run
$FIDDLE\_ROOT/out/fiddle\_main  via ptrace control with fiddle\_secwrap. The
compilation is done via
$FIDDLE\_ROOT/versions/<githash>/cmakeout/skia\_compile\_arguments.txt and
friends. Source images will be loaded in $FIDDLE\_ROOT/images.  The output from
running fiddle\_main is piped to stdout and contains the images as base64
encoded values in JSON. The $FIDDLE\_ROOT/images directory is whitelisted for
access by fiddle\_secwrap so that fiddle\_main can read the source image.

Summary of directories and files and how they are mounted in the container:

Directory                                 | Perms | Container
------------------------------------------|-------|----------
$FIDDLE\_ROOT/goodbuilds.txt               | R     | Y
$FIDDLE\_ROOT/versions/<githash>           | R     | Y
$FIDDLE\_ROOT/versions/<githash>/cmakeout/ | R     | Y
.../cmakeout/fiddle\_main.o                | R     | Y
$FIDDLE\_ROOT/src/                         | R     | Y
$FIDDLE\_ROOT/src/draw.cpp                 | R     | Y
$FIDDLE\_ROOT/tmp/<tmpdir>/                | R     | N
$FIDDLE\_ROOT/out/                         | RW    | Y
$FIDDLE\_ROOT/images/                      | R     | Y
$FIDDLE\_ROOT/bin/fiddle\_secwrap           | R     | Y

Decimation
----------

We could continuously add new builds to /versions/ but each checkout and build
is ~1.3GB. So we'll fill up our 1TB disk in under a year. So we need to keep
around older builds, but can't keep them all. Having finer-grained history for
recent builds is also important, while we can tolerate gaps in older builds.
I.e. we don't really need a build from 30 days ago, and 30 days and 1 hr ago,
but we would like to have almost all of the last weeks worth of commits
available. So we end up with a decimation strategy that is simple but also
accomplishes the above goals.

  * Keep 128+ builds.
  * When the number of builds reaches 257, delete every other build in
    chronological order, that is, order the builds in reverse chronological
    order, number starting at 0, and then delete all the odd builds. Note that
    this preserves the newest build and the oldest build, and again reduces
    the number of builds to 128.

Named Fiddles
-------------
Named fiddles are actually just like soft links from a name to the fiddleHash
of a fiddle. They can only be created by logged in users and the id of the
person that created the named shortcut is attached as metadata to the file.

URLs
----

The URL structure of fiddle is:

    /c/cbb8dee39e9f1576cd97c2d504db8eee - Direct link to a fiddle.

Links to individual resources:

    /i/cbb8dee39e9f1576cd97c2d504db8eee_raster.png
    /i/cbb8dee39e9f1576cd97c2d504db8eee_gpu.png
    /i/cbb8dee39e9f1576cd97c2d504db8eee.pdf
    /i/cbb8dee39e9f1576cd97c2d504db8eee.skp

Links to individual resources for a given commit:

    /ai/<runid>/cbb8dee39e9f1576cd97c2d504db8eee_raster.png
    /ai/<runid>/cbb8dee39e9f1576cd97c2d504db8eee_gpu.png
    /ai/<runid>/cbb8dee39e9f1576cd97c2d504db8eee.pdf
    /ai/<runid>/cbb8dee39e9f1576cd97c2d504db8eee.skp

Where runid is the hash timestamp and git hash of a particular version of Skia.

To create a new fiddle, POST JSON to /\_/run of the form:

    {
      "code":"void draw(SkCanvas...",
      "width":256,
      "height":256,
      "source":0,
    }

Embedding fiddles in iframes is done by:

    /iframe/cbb8dee39e9f1576cd97c2d504db8eee

Which should really just be a version of index.html that strips out much of the
surrounding elements.

Storage
-------

Fiddles are stored in Google Storage under gs://skia-fiddle/, which is
different from fiddle 1.0 where they were stored in MySql. For each fiddle we
store the user's code at:

    gs://skia-fiddle/fiddle/<fiddlehash>/draw.cpp

The image width, height, and source (as a 64bit int) values are stored as metadata on the draw.cpp file.

Note that the fiddlehash must match the hash generated by fiddle 1.0, so that
hash is actually the hash of the user's code with line numbers added, along
with the width and height added in a comment.  We also store the rendered
images as directories below each fiddlehash directory:


    gs://skia-fiddle/fiddle/<fiddlehash>/<ts-hash>-<githash>/cpu.png
    gs://skia-fiddle/fiddle/<fiddlehash>/<ts-hash>-<githash>/gpu.png
    gs://skia-fiddle/fiddle/<fiddlehash>/<ts-hash>-<githash>/skp.skp
    gs://skia-fiddle/fiddle/<fiddlehash>/<ts-hash>-<githash>/pdf.pdf

Note that <ts-hash> is the timestamp of the git commit time in RFC3339 format,
followed by a dash, and then by the githash (revision) of the Skia commit.
This allows the directories to be sorted quickly by name to find the most
recent version of the images, which is what will be displayed by default.

The only other thing that needs to be stored are the source images, which are
stored as files in the /source directory:

    gs://skia-fiddle/source/1
    gs://skia-fiddle/source/2

In addition there is a text file:

    gs://skia-fiddle/source/lastid.txt

That contains in text the largest ID for a source image ever used. This should
be incremented and written back to Google Storage before adding a new image.
Note that writing using generations can prevent the lost update problem.

Named fiddles are actually just like soft links from a name to the fiddleHash
of a fiddle. The named fiddles are stored in:

    gs://skia-fiddle/named/<fiddle name>

Where the id of the fiddle is the filename, and the contents of the file is
the fiddleHash. The id of the person that created the named shortcut is
attached as metadata to the file.

Drive
-----

An attached disk will reside at /mnt/pd0 and will be populated as:

     /mnt/pd0/fiddle  - $FIDDLE_ROOT
     /mnt/pd0/container
     /mnt/pd0/depot_tools

Startup
-------

During instance startup git and systemd-container will be installed and
depot\_tools will also be installed.

The container image and all other exes will be installed via push.

Security
--------

We're putting a C++ compiler on the web, and promising to run the results of
user submitted code, so security is a large concern. Security is handled in a
layered approach, using a combination of seccomp-bpf, chroot jail and rlimits.

seccomp-bpf - Used to limit the types of system calls that the user code can
make. Any attempts to make a system call that isn't allowed causes the
application to terminate immediately. Seccomp-bpf and ptrace are used from
fiddle\_secwrap.cpp.

chroot jail - The code is run in a chroot jail via systemd-nspawn, making the
rest of the operating system files unreachable from the running code.
Systemd-nspawn is launched from fiddle\_run.

rlimits - Used to limit the resources the running code can get access to, for
example runtime is limited to 10s of CPU. The limits are set in fiddle\_run.



