#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Update the root-level buildbot checkout on each buildslave host.

This differs from UpdateScripts in that it updates the root-level buildbot
script checkouts on ALL host machines, as opposed to a single buildslave's
checkout of the buildbot scripts on a single host machine.
"""


import os
import re
import skia_vars
import sys

buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir, os.pardir))
sys.path.append(os.path.join(buildbot_path))

from build_step import BuildStep, BuildStepWarning
from scripts import run_cmd
from utils import force_update_checkout


BUILDBOT_GIT_URL = skia_vars.GetGlobalVariable('buildbot_git_url')


class UpdateAllSlaveHosts(BuildStep):
  def _Run(self):
    script_path = run_cmd.ResolvablePath('slave', 'skia_slave_scripts', 'utils',
                                         'force_update_checkout.py')
    sync_cmd = ['python', script_path]
    results = run_cmd.run_on_all_slave_hosts(sync_cmd)
    failed = []
    for host in results.iterkeys():
      print host,
      # Check and report the results of the command for each build slave host.
      if results[host].returncode != 0:
        # If the command failed (or we failed to log in), print the output and
        # report a failure.
        failed.append(host)
        results[host].print_results(pretty=True)
      else:
        # If the command succeeded, find and print the commit hash we synced
        # to.  If we can't find it, then something must have failed, so
        # print the output and report a failure.
        match = re.search(
            force_update_checkout.GOT_REVISION_PATTERN % ('(\w+)'),
            results[host].stdout)
        if match:
          print '\t%s' % match.group(1)
        else:
          failed.append(host)
          results[host].print_results(pretty=True)

    if failed:
      print
      print 'Failed to update the following hosts:'
      for failed_host in failed:
        print ' ', failed_host

    if failed:
      # TODO(borenet): Make sure that we can log in to all hosts, then make this
      # an error.
      raise BuildStepWarning('Could not update some hosts.')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateAllSlaveHosts))
