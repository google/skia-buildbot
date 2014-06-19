#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Repeatedly launch the build master in an infinite loop, updating the source
between launches. This script is intended to be run at boot time. """


import os
import socket
import subprocess
import sys
import time

BUILDBOT_PATH = os.path.realpath(os.path.join(os.path.dirname(__file__),
                                              os.pardir))
sys.path.append(os.path.join(BUILDBOT_PATH, 'site_config'))
sys.path.append(os.path.join(BUILDBOT_PATH, 'third_party', 'chromium_buildbot',
                             'site_config'))

import config_private


# File where the PID of the running master is stored
PID_FILE = 'twistd.pid'

# Maximum time (in seconds) to wait for PID_FILE to be written after the master
# is launched.  If PID_FILE is not written by then, we assume an error occurred.
PID_TIMEOUT = 10.0


def _SyncSources():
  """ Run 'gclient sync' on the buildbot sources. """
  path_to_gclient = os.path.join(BUILDBOT_PATH, 'third_party', 'depot_tools',
                                 'gclient.py')
  cmd = ['python', path_to_gclient, 'sync']
  if not subprocess.call(cmd) == 0:
    # Don't throw an exception or quit, since we want to keep the master running
    print 'WARNING: Failed to update sources.'


def _LaunchMaster():
  """ Launch the build master and return its PID. """
  # Make sure the master is stopped.
  subprocess.call(['make', 'stop'])

  # Launch the master
  cmd = ['make', 'start']
  if not os.environ.get('TESTING_MASTER'):
    for master in config_private.Master.valid_masters:
      if socket.getfqdn() == master.master_fqdn:
        master_name = master.__name__
        print 'Using master %s' % master_name
        os.environ['TESTING_MASTER'] = master_name
        break
    else:
      print 'Could not find a matching production master. Using default.'
  subprocess.call(cmd)

  # Wait for the pid file to be written, then use it to obtain the master's pid
  pid_file = None
  start_time = time.time()
  while not pid_file:
    try:
      pid_file = open(PID_FILE)
    except Exception:
      if time.time() - start_time > PID_TIMEOUT:
        raise Exception('Failed to launch master.')
      time.sleep(1)
  pid = str(pid_file.read()).rstrip()
  pid_file.close()
  return pid


# TODO(borenet): Share this code with launch_slaves.py.
def IsRunning(pid):
  """ Determine whether a process with the given PID is running.

  pid: string; the PID to test. If pid is None, return False.
  """
  if pid is None:
    return False
  if os.name == 'nt':
    cmd = ['tasklist', '/FI', '"PID eq %s"' % pid]
    output = subprocess.check_output(cmd, stdout=subprocess.PIPE,
                                     stderr=subprocess.STDOUT)
    is_running = pid in output
  else:
    cmd = ['cat', '/proc/%s/stat' % pid]
    is_running = subprocess.call(cmd, stdout=subprocess.PIPE,
                                 stderr=subprocess.STDOUT) == 0
  return is_running


def _BlockUntilFinished(pid):
  """ Blocks until the given process has finished.

  pid: PID of the process to wait for
  """
  while IsRunning(pid):
    time.sleep(1)


def _UpdateAndRunMaster():
  """ Update the buildbot sources and run the build master, blocking until it
  finishes. """
  _SyncSources()
  pid = _LaunchMaster()
  print 'Launched build master with PID: %s' % pid
  _BlockUntilFinished(pid)
  print 'Master process has finished.'


def main():
  """ Alternately sync the buildbot source and launch the build master. """
  loop = '--noloop' not in sys.argv
  master_path = os.path.join(BUILDBOT_PATH, 'master')
  os.chdir(master_path)
  _UpdateAndRunMaster()
  while loop:
    print 'Restarting the build master.'
    _UpdateAndRunMaster()

if '__main__' == __name__:
  sys.exit(main())
