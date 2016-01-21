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


