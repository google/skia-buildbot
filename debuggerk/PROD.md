Debugger Production Manual
========================

First make sure you are familar with the design of the debugger by reading the
[DESIGN](./DESIGN.md) doc.


Forcing a build
---------------

You may need for force a build on the server, say if you accidentally wiped
the mounted drive, or say debugger is broken and you need to force it to build
at HEAD and not wait for that commit to make it into a DEPS rolls.

SSH into `skia-debugger` and run:

    skia_build --force --head --alsologtostderr \
      --depot_tools=/mnt/pd0/depot_tools \
      --work_root=/mnt/pd0/debugger

    cd /mnt/pd0/debugger/versions/[hash of last build]
    ninja -C out/Release skiaserve

After that finishes restart debugger:

    sudo systemctl restart skdebuggerd.service

Or restart debugger from the push dashboard.

Alerts
======

Items below here should include target links from alerts.

build_fail
----------
Debugger is failing to build.

This usually isn't a critical error since Debugger will only start
using a build of Skia if it was successfully built, but this should
be addressed so Debugger doesn't get too far removed from Skia HEAD.

Search logs for "Failed to build LKGR:" and "Successfully built:".

sync_fail
---------
Debugger is failing to sync.

This sync is for Debugger updating a local copy of Skia that's used
to look up git hashes. The repo is located at /mnt/pd0/debugger/skia.

Search logs for "Failed to update skia repo".

One easy fix is to SSH into the machine and delete the directory and
then restart Debugger, which will rebuild the checkout.

