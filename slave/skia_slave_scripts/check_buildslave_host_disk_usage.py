#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Verify that the buildslave host machines are under a disk usage threshold."""


import re
import sys

from build_step import BuildStep, BuildStepWarning, BuildStepFailure
from utils import misc

sys.path.append(misc.BUILDBOT_PATH)

from scripts import run_cmd


MAX_DISK_USAGE_PERCENT = 90


def get_disk_usage_percent(stdout):
  """Parse the disk_usage.py script output and return the disk usage percent.

  Args:
      stdout: string; output from the disk_usage script.
  Returns:
      float; the percentage of disk space used on the machine.
  """
  # The disk_usage.py script's output looks like this:
  # usage(total=382117335040, used=178414583808, free=184575676416)
  total = float(re.findall('total=(\d+)', stdout)[0])
  used = float(re.findall('used=(\d+)', stdout)[0])
  return used / total * 100.0


class CheckSlaveHostsDiskUsage(BuildStep):
  def _Run(self):
    disk_usage_script = run_cmd.ResolvablePath('third_party', 'disk_usage',
                                               'disk_usage.py')
    results = run_cmd.run_on_all_slave_hosts(['python', disk_usage_script])
    failed = []
    over_threshold = False
    print 'Maximum allowed disk usage percent: %d\n' % MAX_DISK_USAGE_PERCENT
    for host in results.iterkeys():
      print host,
      got_result = True
      if results[host].returncode != 0:
        got_result = False
      else:
        try:
          percent_used = get_disk_usage_percent(results[host].stdout)
          print ': %d%%' % percent_used,
          if percent_used > MAX_DISK_USAGE_PERCENT:
            print ' (over threshold)'
            over_threshold = True
          else:
            print
        except (IndexError, ZeroDivisionError):
          got_result = False
      if not got_result:
        failed.append(host)
        print ': failed: ', results[host].stderr

    if failed:
      print
      print 'Failed to get disk usage for the following hosts:'
      for failed_host in failed:
        print ' ', failed_host

    if over_threshold:
      raise BuildStepFailure('Some hosts are over threshold.')

    if failed:
      # TODO(borenet): Make sure that we can log in to all hosts, then make this
      # an error.
      raise BuildStepWarning('Could not log in to some hosts.')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CheckSlaveHostsDiskUsage))
