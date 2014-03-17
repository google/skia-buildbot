#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Update the root-level buildbot checkout on each buildslave host.

This differs from UpdateScripts in that it updates the root-level buildbot
script checkouts on ALL host machines, as opposed to a single buildslave's
checkout of the buildbot scripts on a single host machine.
"""


from utils import gclient_utils
from utils import shell_utils
import os
import shlex
import skia_vars
import sys

buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir, os.pardir))
sys.path.append(os.path.join(buildbot_path))

from build_step import BuildStep, BuildStepWarning
from scripts import run_cmd


BUILDBOT_GIT_URL = skia_vars.GetGlobalVariable('buildbot_git_url')


class UpdateAllSlaveHosts(BuildStep):
  def _Run(self):
    output = shell_utils.run([gclient_utils.GIT, 'ls-remote',
                              BUILDBOT_GIT_URL, '--verify',
                              'refs/heads/master'])
    buildbot_revision = shlex.split(output)[0]

    gclient_path = run_cmd.ResolvablePath('third_party', 'depot_tools',
                                          'gclient')
    sync_cmd = [gclient_path, 'sync', '--force', '--revision',
                'buildbot@%s' % buildbot_revision]
    results = run_cmd.run_on_all_slave_hosts(sync_cmd)
    failed = []
    for host in results.iterkeys():
      print host,
      if results[host].returncode != 0:
        failed.append(host)
        print ':\nstdout:\n%s\nstderr:\n%s\n\n' % (results[host].stdout,
                                                   results[host].stderr)
      else:
        print

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
