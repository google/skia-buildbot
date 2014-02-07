#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on a build slave host machine, listed in slave_hosts_cfg."""


import os
import sys

buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(buildbot_path)

from scripts import local_run
from site_config import slave_hosts_cfg


def run_on_slave_host(slave_host_name, cmd):
  """Run a command on a remote slave host machine, blocking until completion.

  Args:
      slave_host_name: string; name of the slave host machine.
      cmd: list of strings; command to run.
  Returns:
      A dictionary of results with the remote host machine name as its only key
      and individual result dictionaries (with stdout, stderr, and returncode as
      keys) its value.
  """
  login_cmd = slave_hosts_cfg.get_login_command(slave_host_name)
  if not login_cmd:
    raise ValueError('%s does not have a remote login procedure defined in '
                     'slave_hosts_cfg.py.' % slave_host_name)
  host = slave_hosts_cfg.SLAVE_HOSTS[slave_host_name]
  path_to_buildbot = host['path_module'].join(*host['path_to_buildbot'])
  path_to_local_run = host['path_module'].join(path_to_buildbot, 'scripts',
                                               'local_run.py')
  result = local_run.run(login_cmd + ['python', path_to_local_run] + cmd)
  if result['returncode']:
    return { slave_host_name: result }
  return { slave_host_name: local_run.decode_results(result['stdout']) }


if '__main__' == __name__:
  if len(sys.argv) < 3:
    sys.stderr.write('Usage: %s <slave_host_name> <command>\n' % __file__)
    sys.exit(1)
  print local_run.encode_results(run_on_slave_host(sys.argv[1], sys.argv[2:]))
