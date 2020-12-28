General Production Manual
=========================

This file documents things that don't belong to a specific service.

Other Resources (For Googlers only)
-----------------------------------

 - [https://goto.google.com/skia-infra-gardener]
 - [https://goto.google.com/skolo-maintenance]
 - [https://goto.google.com/skolo-playbook]

Alerts
======

Items below here should include target links from alerts.

DiskSpaceLow
------------
This means a given disk on one of our machines has a low disk. Running out of disk space causes
problems, so we try to keep a healthy buffer (which varies depending on the total disk size).
For machines running Swarming, this can cause issues when trying to download a task from Isolate,
which has been a problem before ().

To fix, [connect to the machine](https://skia.org/dev/testing/swarmingbots#connecting-to-swarming-bots),
and use `df -h` or a similar command to identify which disk(s) are low. `du -hd 2` can be a useful
tool for identifying which folders are taking up a lot of space.
 - If a /root disk is full, try cleaning out the APT cache `sudo apt-get clean`
 - If a /var disk is full, try deleting /var/logs/* and restarting the machine.
 - If a /tmp disk is full, it usually cleans itself up on a reboot.
 - On a swarming machine, if /b (/mnt/pd0) is full, there are few things to check:
   - `/b/s/*_cache` folders have gotten very large. If so, stop swarming, delete the folders, and
     reboot.
   - /b/docker (the docker cache) can take up 100+ GB. Clean it with `sudo docker system prune -fa`.

If many machines are experiencing this, you may want to use the
[run_on_swarming_bots](../scripts/run_on_swarming_bots) script to fix them all at once.

Key metrics: collectd_df_df_complex
