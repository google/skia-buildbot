#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Update the buildbot checkouts for each buildslave on every host.

This differs from UpdateScripts in that it updates ALL of the buildbot script
checkouts for ALL buildslaves, as opposed to a single buildslave's checkout of
the buildbot scripts on a single host machine.
"""


import re
import skia_vars
import sys

from build_step import BuildStep, BuildStepWarning
from scripts import run_cmd
from utils import force_update_checkout


BUILDBOT_GIT_URL = skia_vars.GetGlobalVariable('buildbot_git_url')


class UpdateAllBuildslaves(BuildStep):
  def _Run(self):
    script_path = run_cmd.ResolvablePath('slave', 'skia_slave_scripts', 'utils',
                                         'force_update_checkout.py')
    sync_cmd = ['python', script_path]
    results = run_cmd.run_on_all_slaves_on_all_hosts(sync_cmd)
    failed = []
    for host in results.iterkeys():
      print host
      # If results[host] is a MultiCommandResults instance, then we have results
      # for buildslaves running on that machine, which implies that we were able
      # to log in to the machine successfully.
      if isinstance(results[host], run_cmd.MultiCommandResults):
        # We successfully logged into the buildslave host machine.
        for buildslave in results[host].iterkeys():
          print ' ', buildslave,
          # Check and report the results of the command for each buildslave on
          # this host machine.
          if results[host][buildslave].returncode != 0:
            # If the command failed, print its output.
            failed.append(buildslave)
            print
            results[host][buildslave].print_results(pretty=True)
          else:
            # If the command succeeded, find and print the commit hash we synced
            # to.  If we can't find it, then something must have failed, so
            # print the output and report a failure.
            match = re.search(
                force_update_checkout.GOT_REVISION_PATTERN % ('(\w+)'),
                results[host][buildslave].stdout)
            if match:
              print '\t%s' % match.group(1)
            else:
              failed.append(host)
              print
              results[host][buildslave].print_results(pretty=True)
      else:
        # We were unable to log into the buildslave host machine.
        if results[host].returncode != 0:
          failed.append(host)
          results[host].print_results(pretty=True)
      print

    if failed:
      print
      print 'Failed to update the following buildslaves:'
      for failed_host in failed:
        print ' ', failed_host

    if failed:
      # TODO(borenet): Make sure that we can log in to all hosts, then make this
      # an error.
      raise BuildStepWarning('Could not update some buildslaves.')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateAllBuildslaves))
