#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on all build slave host machines listed in slave_hosts_cfg."""


import os
import sys

buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(buildbot_path)

from scripts import local_run
from scripts import run_on_slave_host
from site_config import slave_hosts_cfg


def run_on_all_slave_hosts(cmd):
  """Run the given command on all slave hosts, blocking until completion.

  Args:
      cmd: list of strings; command to run.
  Returns:
      A dictionary of results with host machine names as keys and individual
      result dictionaries (with stdout, stderr, and returncode as keys) as
      values.
  """
  results = {}
  for hostname in slave_hosts_cfg.SLAVE_HOSTS.iterkeys():
    if not slave_hosts_cfg.get_login_command(hostname):
      results.update({hostname:
                      local_run.cmd_results(stderr='No procedure for login.')})
      continue
    results.update(run_on_slave_host.run_on_slave_host(hostname, cmd))
  return results


if '__main__' == __name__:
  if len(sys.argv) < 2:
    sys.stderr.write('Usage: %s <command>\n' % __file__)
    sys.exit(1)
  print local_run.encode_results(run_on_all_slave_hosts(sys.argv[1:]))
