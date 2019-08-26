AutoRoll
========

AutoRoll is a program which creates and manages DEPS rolls of a child project,
eg. Skia, into a parent project, eg. Chrome.


For Sheriffs of Parent Projects
-------------------------------

If a roll has caused a breakage, feel free to revert first and ask questions
later. Generally you should also stop the roller, otherwise rolls will continue
to land and compound the problem. The controls for the roller should be linked
in the roll commit message. It is polite and good practice to directly contact
someone on that team and work with them as needed to get the rolls going agin.

If a roller has gone rogue somehow, eg. uploading too many rolls, chewing up bot
capacity, etc, please stop the roller and file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug
If you need immediate attention, contact skiabot@google.com. Note that we do not
use pagers, and our trooper is generally only active during working hours.


For Child Project Roller Owners
-------------------------------

In the case of any problems or unexpected behavior, please stop the roller and
file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug
If you need immediate attention, contact skiabot@google.com. Note that we do not
use pagers, and our trooper is generally only active during working hours.

If rolls are failing due to a breakage in the parent repo, you generally do not
need to stop the roller unless you are concerned about saving commit queue
capacity. If rolls are failing due to a broken commit in the child repo, use
your judgment as to whether to stop the roller; it isn't strictly required,
since the roller will continue to retry as new commits land, but it does save
commit queue capacity if you know that the rolls are doomed to fail until a fix
or revert lands.

Stopping a roller causes any active roll to be abandoned. You can take advantage
of this behavior if you know that the current roll is doomed to fail and the
next will contain a fix: rather than waiting for the commit queue to fail, stop
the roller, wait for the current roll to be abandoned, and resume the roller.


AutoRoll Modes
--------------

There are three modes in which the roller may run:


#### Running ####

This is the "normal" mode. The roller will upload DEPS roll CLs, close those
which fail, and upload new CLs until the child repo is up-to-date in the parent
repo's DEPS.


#### Stopped ####

The roller will not upload any CLs. Any in-progress roll CL will be closed when
the roller is stopped.


#### Dry Run ####

The roller will upload CLs and run the commit queue dry run. If the
dry run succeeds, the CL is left open until either the roller is set back to
"running" mode, in which case the CL re-enters the commit queue, or until one
or more commits lands in the child repo, in which case the roller closes the
CL and uploads a new one.


Troubleshooting for Skia Infra Team Members
-------------------------------------------

#### The roller has a high error rate ####

Find the logs for the roller to see what's going on. The most common cause of
this alert is transient failures while communicating with Git or Gerrit. In that
case, there is not much which can be done other than to silence the alert and
hope that things improve. If the failure persists, contact the current on-call
for the Git service.


#### The autoroll page displays "Status: error" and doesn't upload CLs ####

Some issue is preventing the roller from running normally. You'll need to look
through the logs for more information. Normally this is transient, but it can
also be caused by mis-configuration of the roller, eg. the configured reviewer
is not a committer.


#### Something is wrong, and I need to shut down the roller ASAP! ####

Setting the roller mode to "Stopped" should be enough in most cases. If not,
you can use `kubectl delete -f <file>` to kill it.
