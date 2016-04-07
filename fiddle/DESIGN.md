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
  * fiddle_secwrap - Runs a program under the control of ptrace.
  * fiddle_main - The user's code with a wrapper to load a source
                  image and to write out the resulting PNGs, PDF,
                  an SKP data.
  * fiddle_run - This is run within the chroot jail. It compiles
                 the user's code against fiddle_main.o and
                 libskia.so and then runs the resulting executable
                 under the control of fiddle_secwrap. It gathers
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


By default $FIDDLE_ROOT is /mnt/pd0, but can be another directory when running
locally and not using systemd-nspawn.

Skia is checked out into $FIDDLE_ROOT/versions/<githash>, and cmake built,
with the output going into $FIDDLE_ROOT/versions/<githash>/cmakeout.

The rest of the work, compiling the user's code and then running it, is done
in the container, i.e. run in a root jail using systemd-nspawn.

In the container, / is mounted read-only. Also bind a directory
$FIDDLE_ROOT/src/ as read-only, where the source for $FIDDLE_ROOT/src/ is
$FIDDLE_ROOT/tmp/<tmpdir>/, where tmpdir is unique for each requested compile.
(This is just a symbolic link when not running via nspawn.) Also mount a tmpfs
at $FIDDLE_ROOT/out for the executable output of the compile, this will just
be a tmpfs in the container so it gets cleaned up when the container exits.
Compile $FIDDLE_ROOT/src/draw.cpp and run $FIDDLE_ROOT/out/fiddle_main  via
ptrace control with fiddle_secwrap. The compilation is done via
$FIDDLE_ROOT/versions/<githash>/cmakeout/skia_compile_arguments.txt and
friends. Source images will be loaded in $FIDDLE_ROOT/images.  The output from
running fiddle_main is piped to stdout and contains the images as base64
encoded values in JSON. The $FIDDLE_ROOT/images directory is whitelisted for
access by fiddle_secwrap so that fiddle_main can read the source image.

Summary of directories and how they are mounted in the container:

Directory                                 | Perms | Container
------------------------------------------|-------|----------
$FIDDLE_ROOT/versions/<githash>           | R     | Y
$FIDDLE_ROOT/versions/<githash>/cmakeout/ | R     | Y
.../cmakeout/fiddle_main.o                | R     | Y
$FIDDLE_ROOT/src/                         | R     | Y
$FIDDLE_ROOT/src/draw.cpp                 | R     | Y
$FIDDLE_ROOT/tmp/<tmpdir>/                | R     | N
$FIDDLE_ROOT/out/                         | RW    | Y
$FIDDLE_ROOT/images/                      | R     | Y
$FIDDLE_ROOT/bin/fiddle_secwrap           | R     | Y


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

Security
--------

We're putting a C++ compiler on the web, and promising to run the results of
user submitted code, so security is a large concern. Security is handled in a
layered approach, using a combination of seccomp-bpf, chroot jail and rlimits.

seccomp-bpf - Used to limit the types of system calls that the user code can
make. Any attempts to make a system call that isn't allowed causes the
application to terminate immediately.

chroot jail - The code is run in a chroot jail via systemd-nspawn, making the rest of the
operating system files unreachable from the running code.

rlimits - Used to limit the resources the running code can get access to, for
example runtime is limited to 5s of CPU.



