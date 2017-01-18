Cluster Telemetry Master
=======================

Instance creation and deletion scripts for Cluster Telemetry Master. The CT
master polls ct.skia.org and picks up tasks.
After tasks are picked up, it builds the corresponding master script and
executes the swarming task.

Note that there aren't any setup or push scripts as all of that is handled by
Skia Push. See ../../push/DESIGN.md.

