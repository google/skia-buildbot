#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on a build slave host machine, listed in slave_hosts_cfg."""


import os
import subprocess
import sys

buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(buildbot_path)

from site_config import slave_hosts_cfg


def run_on_slave_host(slave_host_name, cmd):
  """Run the given command on the given slave host machine.

  slave_host_name: string; name of the slave host machine.
  cmd: list of strings; command to run.
  """
  login_cmd = slave_hosts_cfg.get_login_command(slave_host_name)
  if not login_cmd:
    raise ValueError('%s does not have a remote login procedure defined in '
                     'slave_hosts_cfg.py.' % slave_host_name)
  subprocess.check_call(login_cmd + cmd)


if '__main__' == __name__:
  if len(sys.argv) < 3:
    sys.stderr.write('Usage: %s <slave_host_name> <command>\n' % __file__)
    sys.exit(1)
  run_on_slave_host(sys.argv[1], sys.argv[2:])
