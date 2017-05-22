~~~~
    skia-fiddle
    +----------------------------------------------------------+
    |                                                          |
    |                                    gn/ninja              |
    |  fiddle_root/versions/<githash>/  +-----> libskia.a      |
    |  fiddle_root/.../fiddle_main.cpp  +-----> fiddle         |
    |                                                          |
    |                                                          |
    |                                                          |
    |  user's code written and mounted in overlayfs to         |
    |  overwrite the default tools/fiddle/draw.cpp.            |
    |                                                          |
    |                                                          |
    |  overlayfs                                               |
    |    +                                                     |
    |    |                                                     |
    |    +-> fiddle_run (stdout produces json)                 |
    |           +       (capture stdout/stderr of child procs) |
    |           |                                              |
    |           |                   ninja                      |
    |           +-> draw.cpp       +----->  fiddle             |
    |           |                                              |
    |           |                                              |
    |           |                                              |
    |           +-> fiddle_secwrap                             |
    |                   +                                      |
    |                   |                                      |
    |                   +-> fiddle                             |
    |                                                          |
    |                                                          |
    +----------------------------------------------------------+
~~~~

Directory                                            | Description
-----------------------------------------------------|-----------
$fiddle\_root/goodbuilds.txt                         | Good git hashes.
$fiddle\_root/<githash>/skia/tools/fiddle/draw.cpp   | Original to hide.
$fiddle\_root/tmp/<runid>/                           | Dir per run.
$fiddle\_root/tmp/<runid>/skiawork/                  | Temp file for overlayfs.
$fiddle\_root/tmp/<runid>/                           |
            ./skiaupper/skia/tools/fiddle/draw.cpp   | Overwrites orig draw.cpp.
$fiddle\_root/tmp/<runid>/overlay/                   | The overlay fs.
$fiddle\_root/images/                                | Source images.
$fiddle\_root/bin/fiddle\_secwrap                    | fiddle\_secwrap.

Each run has a unique id associated with it, that id used to segregate the
overlay file systems created to handle that run.

mkdir /mnt/pd0/tmp/<runid>/
mkdir /mnt/pd0/tmp/<runid>/skiawork
mkdir /mnt/pd0/tmp/<runid>/skiaupper/skia/tools/fiddle/
cp draw.cpp /mnt/pd0/tmp/<runid>/skiaupper/skia/tools/fiddle/draw.cpp

export UPPER=/mnt/pd0/tmp/<runid>/skiaupper
export WORK=/mnt/pd0/tmp/<runid>/skiawork
export LOWER=$fiddle\_root/versions/<githash>
mount -t overlay -o lowerdir=$LOWER,upperdir=$UPPER,workdir=$WORK /mnt/pd0/tmp/<runid>/overlay

// When done:
umount /mnt/pd0/tmp/<runid>/overlay
rmdir /mnt/pd0/tmp/<runid>


