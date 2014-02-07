#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on all buildslaves on this machine."""


import os
import socket
import sys


buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(os.path.join(buildbot_path))

from scripts import local_run
from site_config import slave_hosts_cfg


def run_on_slaves(cmd):
  """Run the command on each local buildslave, blocking until completion.

  Args:
      cmd: list of strings; the command to run.
  Returns:
      A dictionary of results with buildslave names as keys and individual
      result dictionaries (with stdout, stderr, and returncode as keys) as
      values.
  """
  slave_host = slave_hosts_cfg.GetSlaveHostConfig(socket.gethostname())
  slaves = slave_host['slaves']
  results = {}
  for (slave, _) in slaves:
    os.chdir(os.path.join(buildbot_path, slave, 'buildbot'))
    res = local_run.run(cmd)
    results[slave] = res
  return results


if '__main__' == __name__:
  print local_run.encode_results(run_on_slaves(sys.argv[1:]))
