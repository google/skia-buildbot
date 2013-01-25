#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Repeatedly launch the build master in an infinite loop, updating the source
between launches. This script is intended to be run at boot time. """


import os
import subprocess
import sys
import time


# File where the PID of the running master is stored
PID_FILE = 'twistd.pid'

# Maximum time (in seconds) to wait for PID_FILE to be written after the master
# is launched.  If PID_FILE is not written by then, we assume an error occurred.
PID_TIMEOUT = 10.0


def _SyncSources():
  """ Run 'gclient sync' on the buildbot sources. """
  path_to_gclient = os.path.join(os.pardir, os.pardir, 'depot_tools',
                                 'gclient.py')
  cmd = ['python', path_to_gclient, 'sync']
  if not subprocess.call(cmd) == 0:
    # Don't throw an exception or quit, since we want to keep the master running
    print 'WARNING: Failed to update sources.'


def _LaunchMaster(private=False):
  """ Launch the build master and return its PID.

  private: boolean designating whether or not to set the master as private.
  """
  # Make sure the master is stopped.
  cmd = ['make', 'stop']
  kill_proc = subprocess.Popen(cmd)
  kill_proc.wait()

  # Launch the master
  cmd = ['make', 'start']
  env = dict(os.environ)
  if private:
    env['PRIVATE_MASTER'] = 'True'
  launch_proc = subprocess.Popen(cmd, env=env)
  launch_proc.wait()

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
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT)
    if proc.wait() != 0:
      raise Exception('Unable to poll process with PID %s' % pid)
    is_running = pid in proc.communicate()[0]
  else:
    cmd = ['cat', '/proc/%s/stat' % pid]
    is_running = subprocess.Popen(cmd, stdout=subprocess.PIPE,
                                  stderr=subprocess.STDOUT).wait() == 0
  return is_running


def _BlockUntilFinished(pid):
  """ Blocks until the given process has finished.

  pid: PID of the process to wait for
  """
  while IsRunning(pid):
    time.sleep(1)


def _UpdateAndRunMaster(private=False):
  """ Update the buildbot sources and run the build master, blocking until it
  finishes.
  
  private: boolean designating whether or not to set the master as private.
  """
  _SyncSources()
  pid = _LaunchMaster(private=private)
  print 'Launched build master with PID: %s' % pid
  _BlockUntilFinished(pid)
  print 'Master process has finished.'


def main():
  """ Alternately sync the buildbot source and launch the build master. """
  private = '--private' in sys.argv
  loop = '--loop' in sys.argv
  master_path = os.path.join(os.path.split(os.path.abspath(__file__))[0],
                             os.pardir, 'master')
  os.chdir(master_path)
  _UpdateAndRunMaster(private=private)
  while loop:
    print 'Restarting the build master.'
    _UpdateAndRunMaster(private=private)

if '__main__' == __name__:
  sys.exit(main())
