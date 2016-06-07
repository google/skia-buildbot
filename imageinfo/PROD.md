ImageInfo Production Manual
========================

First make sure you are familar with the design of imageinfo by reading the
[DESIGN](./DESIGN.md) doc.


Forcing a build
---------------

You may need for force a build on the server, say if you accidentally wiped
the mounted drive, or say imageinfo is broken and you need to force it to build
at HEAD and not wait for that commit to make it into a DEPS rolls.

SSH into `skia-imageinfo` and run:

    skia_build --force --head --alsologtostderr \
      --depot_tools=/mnt/pd0/depot_tools \
      --work_root=/mnt/pd0/imageinfo

    cd /mnt/pd0/imageinfo/versions/[hash of last build]
    ninja -C  out/Release visualize_color_gamut


After that finishes restart imageinfo:

    sudo systemctl restart imageinfo.service

Or restart ImageInfo from the push dashboard.

Alerts
======

Items below here should include target links from alerts.

build_fail
----------
ImageInfo is failing to build.

This usually isn't a critical error since ImageInfo will only start
using a build of Skia if it was successfully built, but this should
be addressed so ImageInfo doesn't get too far removed from Skia HEAD.

Search logs for "Failed to build LKGR:" and "Successfully built:".

sync_fail
---------
ImageInfo is failing to sync.

This sync is for ImageInfo updating a local copy of Skia that's used
to look up git hashes. The repo is located at /mnt/pd0/imageinfi/skia.

Search logs for "Failed to update skia repo".

One easy fix is to SSH into the machine and delete the directory and
then restart ImageInfo, which will rebuild the checkout.

