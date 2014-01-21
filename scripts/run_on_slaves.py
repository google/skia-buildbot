#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on all buildslaves on this machine."""


import os
import socket
import subprocess
import sys


buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(os.path.join(buildbot_path, 'site_config'))


import slave_hosts_cfg


def run_on_slaves(cmd):
  """Run the command on each buildslave on this machine.

  Args:
      cmd: string or list of strings; the command to run.
  """
  slave_host = slave_hosts_cfg.GetSlaveHostConfig(socket.gethostname())
  slaves = slave_host['slaves']
  failed = []
  for (slave, _) in slaves:
    print 'cd %s' % os.path.join(buildbot_path, slave, 'buildbot')
    print cmd
    os.chdir(os.path.join(buildbot_path, slave, 'buildbot'))
    try:
      subprocess.check_call(cmd)
    except subprocess.CalledProcessError:
      failed.append(slave)


if '__main__' == __name__:
  run_on_slaves(sys.argv[1:])
