AutoRoll
========

AutoRoll is a program which creates and manages DEPS rolls of a child project,
eg. Skia, into a parent project, eg. Chrome.


For Gardeners of Parent Projects
-------------------------------

If a roll has caused a breakage, feel free to revert first and ask questions
later. Generally you should stop the roller first, otherwise rolls will continue
to land and compound the problem. The controls for the roller should be linked
in the roll commit message. It is polite and good practice to directly contact
someone on that team which owns the roller and work with them as needed to get
the rolls going again.

If a roller has gone rogue somehow, eg. uploading too many rolls, chewing up bot
capacity, etc, please stop the roller and file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug
If you need immediate attention, contact skiabot@google.com. Note that we do not
use pagers, and our gardener is generally only active during working hours.


For Child Project Roller Owners
-------------------------------

In the case of any problems or unexpected behavior, please stop the roller and
file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug
If you need immediate attention, contact skiabot@google.com. Note that we do not
use pagers, and our gardener is generally only active during working hours.

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


Configuration
-------------

Configuration files for each of the autorollers may be found
[here](https://skia.googlesource.com/skia-autoroll-internal-config) (Googlers
only).  Feel free to make a CL to modify a roller config, or
[file a bug](https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug)
to request a change.


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


AutoRoll Strategies
-------------------

There are three strategies which the roller may use to choose the next revision
to roll:


#### Batch ####

Using this strategy, the roller always chooses the most recent revision to roll,
potentially resulting in large batches of commits in each roll.


#### N-Batch ####

Similar to the "batch" strategy, the roller will upload rolls containing
multiple commits, but only up to a maximum of N commits, where N is hard-coded
to 20 as of May 2 2022.


#### Single ####

The roller will only roll a single commit at a time.  This can be useful for
keeping blamelists clear, but it has some drawbacks.  If the commit queue is not
fast enough to keep up with the commit rate of the child project, the roller
will lag behind.  If a particular commit breaks the commit queue, the roller may
get stuck, since it won't automatically include the fix or revert in the rolls.
Therefore, this strategy may require occasional manual intervention.


For Skia Infra Team Members
---------------------------

See [PROD.md] (https://skia.googlesource.com/buildbot/+doc/master/autoroll/PROD.md)
for information about handling alerts.
