Skia Debugger
=============

The Skia Debugger consists of two components, the C++ command-line application
that ingests SKPs and analyzes them, and the HTML/CSS/JS front-end to the
debugger that is loaded off of https://debugger.skia.org.

The C++ command-line application embeds a web server and provides a web API
for inspecting and interacting with an SKP that the front-end consumes.

Normally the skdebugger will be run on the same machine that the browser is
run on:

~~~~
                          +---------------------+
                          |                     |
                          |  debugger.skia.org  |
                          |                     |
                          +----+----------------+
                               ^
                               |
+-----------------------------------------------+
|                              |                |
| Desktop                      |                |
|            +-----------+     |                |
|            |           |     |  +---------+   |
|            |  Browser  +-----+  |         |   |
|            |           |        | skdebug |   |
|            |           +------> |         |   |
|            +-----------+        +---------+   |
|                                               |
|                                               |
+-----------------------------------------------+
~~~~

But that ins't a requirement, and the skdebugger could be run on a remove
machine, or with port forwarding could be run on an Android device:

~~~~
                          +---------------------+
                          |                     |
                          |  debugger.skia.org  |
                          |                     |
                          +----+----------------+
                               ^
                               |
+----------------------------------+    +----------------------+
|                              |   |    |                      |
| Desktop                      |   |    |  Android Device      |
|            +-----------+     |   |    |                      |
|            |           |     |   |    |        +---------+   |
|            |  Browser  +-----+   |    |        |         |   |
|            |           |         |    |        | skdebug |   |
|            |           +---------------------->+         |   |
|            +-----------+         |    |        +---------+   |
|                                  |    |                      |
|                                  |    |                      |
+----------------------------------+    +----------------------+
~~~~

URL Structure
=============

Current Actions:
  * Upload a new SKP.
  * Get info about the SKP.
  * Get the rendered image of the SKP.

Future Actions:
  * Get the rendered image of an SKP up to command N (N=0 to number of total commands in SKP - 1)
  * Get the current info (matrix, clip) for an SKP up to command N.
  * Modify command N (change values, not the command itself).

Far Future Actions:
  * Insert command at N.
  * Delete command N.
  * Update command N (change both values and/or command).


    /new
      POST /new - Start working on a new SKP. The content is a
          multipart/form-data with the SKP uploaded as 'file'.

    /info[/N]
      GET /info - Get general info for the fully rendered SKP (matrix, clip).
      Get /info/N - Get info about the SKP after rendering to command N (matrix, clip).

    /img[/N]
      Get /img - Get the rendered image from the full SKP.
      Get /img/N - Get the rendered image up to command N.

    /cmd[/N][/toggle]
      GET /cmd - Returns JSON description of all commands.
      GET /cmd/N - Returns JSON description of one command.
      PUT /cmd/N - Update the command at location N.
      DELETE /cmd/N - Delete command at location N.
      POST /cmd/N/[0|1] - Toggles command N on or off.

    /clipAlpha/[0-255]
      POST - Change the opacity of the clip overlay.

    /break/n/x/y
      GET - Returns the index of the next op after 'n'
        where the color of the pixel at (x, y) has changed.

    /enableGPU/[0|1]
      POST - Changes the rendering to/from CPU/GPU.

    /colorMode/[0|1|2]
      POST - Changes rendering to Linear 32-bit (0), sRGB (1), or Half-float (2)

Hosted Debugger
---------------

A hosted version of the debugger runs on debugger.skia.org.

Each signed in user has a skiaserve that is run in a chroot jail just for
them, so these are long running processes. Each on runs on a different port
and the skdebugger Go app proxies requests to different skiaserve instances
based on the users id.

Every hour the server attempts to build skiaserve at LKGR. When a new
skiaserve instance is started it always uses the latest LKGR of skiaserve.

An attached disk will reside at /mnt/pd0 and will be populated as:

     /mnt/pd0/container    - Image for chroot jail.
     /mnt/pd0/depot_tools  - A copy of depot_tools.
     /mnt/pd0/debugger     - $WORK_ROOT
     /mnt/pd0/debugger/skia - A checkout of Skia used only for
                              look up git commit hashes.
     /mnt/pd0/debugger/versions/[git hash] - Checkouts of
                            Skia at various LKGRs.

~~~~
    skia-debugger
    +----------------------------------------------------------+
    |                                                          |
    |                                    gyp/ninja             |
    |  WORK_ROOT/versions/<githash>/  +------------> skiaserve |
    |                                                          |
    |                                                          |
    |  systemd-nspawn                                          |
    |    +                                                     |
    |    |                                                     |
    |    +-> skiaserve (opens TCP/IP port)                     |
    |                                                          |
    |                                                          |
    +----------------------------------------------------------+
~~~~


Decimation
----------

We could continuously add new builds to /versions/ but each checkout and build
is ~1.3GB. So we'll fill up our 1TB disk in under a year. So we need to keep
around older builds, but can't keep them all. Having finer-grained history for
recent builds is also important, while we can tolerate gaps in older builds.
I.e. we don't really need a build from 30 days ago, and 30 days and 1 hr ago,
but we would like to have almost all of the last weeks worth of commits
available. So we end up with a decimation strategy that is simple but also
accomplishes the above goals. For example:

  * Keep N/2 or more builds.
  * Preserve all builds that are spaced one month apart.
  * If there are more than N remaining builds (after removing
    from consideration the builds that are one month apart)
    remove every other one to bring the count down to N/2.
