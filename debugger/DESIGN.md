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

