AutoRoll
========

AutoRoll is a program which creates and manages DEPS rolls of a child projects,
eg. Skia, into a parent project, eg. Chrome.


AutoRoll Modes
--------------

There are three modes in which the roller may run:


#### Running ####

This is the "normal" state. The roller will upload DEPS roll CLs,
close those which fail, and upload new CLs until the child repo is up-to-date
in the parent repo's DEPS.


#### Stopped ####

The roller will not upload any CLs. Any in-progress roll CL when the
roller is stopped will be closed. Generally, you don't need to stop the roller
when rolls are failing due to a broken code change in the child repo, as long
as a fix or revert is coming soon.


#### Dry Run ####

The roller will upload CLs and run the commit queue dry run. If the
dry run succeeds, the CL is left open until either the roller is set back to
"running" mode, in which case the CL re-enters the commit queue, or until one
or more commits lands in the child repo, in which case the roller closed the
CL and uploads a new one.


Troubleshooting
---------------

#### The autoroll page displays "Status: error" and doesn't upload CLs ####

Some issue is preventing the roller from running normally. You'll need to look
through the logs for more information. Visit [Skia Push](https://push.skia.org),
find the appropriate GCE instance and follow the log links to find more
information.


#### Something is wrong, and I need to shut down the roller ASAP! ####

Setting the roller mode to "Stopped" should be enough in most cases. If not,
visit [Skia Push](https://push.skia.org), find the appropriate GCE instance
and use the button to stop the autorolld process.

