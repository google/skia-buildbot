Fiddle Production Manual
========================

First make sure you are familar with the design of fiddle by reading the
[DESIGN](./DESIGN.md) doc.


Forcing a build
---------------

You may need for force a build on the server, say if you accidentally wiped
the mounted drive, or say fiddle is broken and you need to force it to build
at HEAD and not wait for that commit to make it into a DEPS rolls.

SSH into `skia-fiddle` and run:

    fiddle_build --force --head --alsologtostderr \
      --depot_tools=/mnt/pd0/depot_tools \
      --fiddle_root=/mnt/pd0/fiddle

After that finishes restart fiddle:

    sudo systemctl restart fiddle.service

Or restart Fiddle from the push dashboard.

Debugging
=========

You can add the `--preserveTemp` flag to `fiddle.system` and that will cause
the temp directories created to store the code and final fiddle executable to
be preserved which may make debugging easier.

Debugging fiddle\_secwrap.cpp
-----------------------------

Highly unlikely to be needed, but if font handling changes, for example, then
Skia applications may start trying to read new directories or make exciting
new system calls.

If that happens then uncomment the line:

        TRACE_ALL,

in `fiddle_secwrap.cpp`, then compile and run fiddle\_secwrap locally and then
run it over the offending exe to determine which calls it is making and then
add those to the whitelist.

Alerts
======

Items below here should include target links from alerts.

Fiddle is failing to build <a id=build_fail></a>
------------------------------------------------

This usually isn't a critical error since Fiddle will only start
using a build of Skia if it was successfully built, but this should
be addressed so Fiddle doesn't get too far removed from Skia HEAD.

Search logs for "Failed to build LKGR:" and "Successfully built:".

Fiddle is failing to sync<a id=sync_fail></a>
------------------------------------------------

This sync is for Fiddle updating a local copy of Skia that's used
to look up git hashes. The repo is located at /mnt/pd0/fiddle/skia.

Search logs for "Failed to update skia repo".

One easy fix is to SSH into the machine and delete the directory and
then restart Fiddle, which will rebuild the checkout.
