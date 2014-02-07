#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command and report its results in machine-readable format."""


import json
import os
import socket
import subprocess
import sys

buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(os.path.join(buildbot_path))

from site_config import slave_hosts_cfg

# We print this string before and after the important output from the command.
# This makes it easy to ignore output from SSH, shells, etc.
BOOKEND_STR = '@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@'


def encode_results(results):
  """Convert a dictionary of results into a machine-readable string.

  Args:
      results: dict, the results of a command.
  Returns:
      A JSONified string, bookended by BOOKEND_STR for easy parsing.
  """
  return (BOOKEND_STR + json.dumps(results) +
          BOOKEND_STR).decode('string-escape')


def decode_results(results_str):
  """Convert a machine-readable string into a dictionary of results.

  Args:
      results_str: string; output from "run" or one of its siblings.
  Returns:
      A dictionary of results.
  """
  return json.loads(results_str.split(BOOKEND_STR)[1])


def cmd_results(stdout='', stderr='', returncode=1):
  """Create a results dict for a command.

  Args:
      stdout: string; stdout from a command.
      stderr: string; stderr from a command.
      returncode: string; return code of a command.
  """
  return {'stdout': stdout.encode('string-escape'),
          'stderr': stderr.encode('string-escape'),
          'returncode': returncode}


def run(cmd):
  """Run the command, block until it completes, and return a results dictionary.

  Args:
      cmd: string or list of strings; the command to run.
  Returns:
      A dictionary with stdout, stderr, and returncode as keys.
  """
  try:
    proc = subprocess.Popen(cmd, shell=False, stderr=subprocess.PIPE,
                            stdout=subprocess.PIPE)
  except OSError as e:
    return cmd_results(stderr=str(e))
  stdout, stderr = proc.communicate()
  return cmd_results(stdout=stdout,
                     stderr=stderr,
                     returncode=proc.returncode)


def run_on_local_slaves(cmd):
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
    res = run(cmd)
    results[slave] = res
  return results


def run_on_remote_host(slave_host_name, cmd):
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
  path_to_run_cmd = host['path_module'].join(path_to_buildbot, 'scripts',
                                               'run_cmd.py')
  result = run(login_cmd + ['python', path_to_run_cmd] + cmd)
  if result['returncode']:
    return { slave_host_name: result }
  return { slave_host_name: decode_results(result['stdout']) }


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
                      cmd_results(stderr='No procedure for login.')})
      continue
    results.update(run_on_remote_host(hostname, cmd))
  return results


if '__main__' == __name__:
  print encode_results(run(sys.argv[1:]))
